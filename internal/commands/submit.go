package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootcraft-cn/cli/internal/archive"
	"github.com/bootcraft-cn/cli/internal/client"
	"github.com/bootcraft-cn/cli/internal/config"
	"github.com/bootcraft-cn/cli/internal/ui"

	"gopkg.in/yaml.v3"
)

type bootcraftMeta struct {
	Course   string `yaml:"course"`
	Language string `yaml:"language"`
}

func SubmitCommand(args []string) error {
	flags := flag.NewFlagSet("submit", flag.ContinueOnError)
	stage := flags.String("stage", "", "指定评测关卡 (slug)")
	message := flags.String("message", "", "自定义提交备注")
	dryRun := flags.Bool("dry-run", false, "仅预览打包文件，不上传")
	force := flags.Bool("force", false, "跳过未提交变更确认")
	apiURL := flags.String("api-url", "", "API 地址（内部测试用）")
	if err := flags.Parse(args); err != nil {
		return err
	}

	// 1. Find bootcraft.yml
	meta, projectDir, err := findBootcraftConfig()
	if err != nil {
		return err
	}

	// 2. Auth
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}
	token := cfg.GetToken()
	if token == "" {
		return errors.New("未登录，请先运行: bootcraft login")
	}

	baseURL := cfg.GetAPIURL(*apiURL)
	c := client.New(baseURL, token)

	// 3. Git commit info
	commitSHA := runGit(projectDir, "rev-parse", "HEAD")
	commitMsg := runGit(projectDir, "log", "-1", "--format=%s")
	if *message != "" {
		commitMsg = *message
	}

	// 4. Check uncommitted changes
	if commitSHA != "" {
		dirty := runGit(projectDir, "status", "--porcelain")
		if dirty != "" {
			ui.Warn("⚠ 检测到未提交变更，提交结果将不关联 git commit 记录。建议先 git commit 后再提交。")
			if !*force && ui.IsTTY() {
				if !ui.Confirm("继续提交？[y/N]") {
					return errors.New("已取消")
				}
			}
			commitSHA = ""
			if *message == "" {
				commitMsg = ""
			}
		}
	}

	// 5. Pack
	ui.Print("📦 打包代码中...")
	buf, fileCount, totalSize, err := archive.Pack(projectDir)
	if err != nil {
		return fmt.Errorf("打包失败: %w", err)
	}
	ui.Printf(" (%d 个文件, %s)\n", fileCount, formatBytes(totalSize))

	// Client-side pre-checks
	const maxFileCount = 200
	const maxTotalSize = 8 * 1024 * 1024
	const maxCompressed = 2 * 1024 * 1024
	if fileCount > maxFileCount {
		return fmt.Errorf("文件数量超限（%d > %d），请检查 .gitignore / .bootcraftignore", fileCount, maxFileCount)
	}
	if totalSize > maxTotalSize {
		return fmt.Errorf("代码包解压后大小超限（%s > 8MB），请排除不必要的文件", formatBytes(totalSize))
	}
	if int64(buf.Len()) > maxCompressed {
		return fmt.Errorf("代码包压缩后大小超限（%s > 2MB），请排除不必要的文件", formatBytes(int64(buf.Len())))
	}

	if *dryRun {
		ui.Println("[dry-run] 仅预览，不上传")
		return nil
	}

	// 6. Upload
	ui.Print("🚀 上传到评测服务...")
	submitResp, err := c.Submit(client.SubmitParams{
		Course:        meta.Course,
		Language:      meta.Language,
		Stage:         *stage,
		Archive:       buf,
		CommitSHA:     commitSHA,
		CommitMessage: commitMsg,
	})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			switch {
			case apiErr.StatusCode == http.StatusUnauthorized || apiErr.Code == "UNAUTHORIZED":
				return errors.New("\nToken 已失效，请重新登录: bootcraft login")
			case apiErr.Code == "REPO_NOT_FOUND":
				return fmt.Errorf("\n未找到仓库记录（课程: %s，语言: %s）\n请先前往 https://www.bootcraft.cn 创建该课程的仓库，再重新提交", meta.Course, meta.Language)
			}
		}
		return fmt.Errorf("\n上传失败: %w", err)
	}
	ui.Println(" 完成")
	ui.Printf("📋 Stage: %s「%s」\n", submitResp.StageSlug, submitResp.StageName)

	// 7. Watch evaluation
	result, skipLogs, err := watchSubmission(c, submitResp.SubmissionID)
	if err != nil {
		return err
	}

	// 8. Render result
	return renderResult(result, skipLogs)
}

func watchSubmission(c *client.Client, submissionID string) (*client.SubmissionStatusResponse, bool, error) {
	tokenResp, err := c.GetTriggerToken(submissionID)
	if err != nil {
		ui.Warn("实时日志不可用，切换到轮询模式")
		result, err := pollSubmission(c, submissionID)
		return result, false, err
	}

	result, streamErr := streamEvalLogs(c, submissionID, tokenResp.TriggerRunID, tokenResp.PublicAccessToken)
	if streamErr != nil {
		ui.Warn("SSE 连接中断，切换到轮询模式")
		result, err := pollSubmission(c, submissionID)
		return result, false, err
	}
	return result, true, nil
}

func streamEvalLogs(c *client.Client, submissionID, runID, accessToken string) (*client.SubmissionStatusResponse, error) {
	// Context cancelled 3s after evaluation completes, giving SSE time to flush remaining logs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalResult *client.SubmissionStatusResponse
	go func() {
		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(3 * time.Second)
			status, err := c.GetSubmissionStatus(submissionID)
			if err != nil {
				continue
			}
			if client.IsTerminalStatus(status.Status) {
				finalResult = status
				// Grace period: let SSE flush remaining log chunks before cancelling
				time.Sleep(3 * time.Second)
				cancel()
				return
			}
		}
		cancel()
	}()

	sseURL := fmt.Sprintf("https://api.trigger.dev/realtime/v1/streams/%s/eval-logs", runID)
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	sseClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := sseClient.Do(req)
	if err != nil && ctx.Err() == nil {
		return nil, err
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("SSE status %d", resp.StatusCode)
		}
		spinner := ui.NewSpinner("评测中")
		firstLine := true
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				if firstLine {
					spinner.Stop()
					firstLine = false
				}
				chunk := strings.TrimPrefix(line, "data: ")
				var text string
				if jsonErr := json.Unmarshal([]byte(chunk), &text); jsonErr == nil {
					fmt.Print(text)
				} else {
					fmt.Print(chunk)
				}
			}
		}
		if firstLine {
			spinner.Stop()
		}
	}

	if finalResult != nil {
		return finalResult, nil
	}
	return c.GetSubmissionStatus(submissionID)
}

func pollSubmission(c *client.Client, submissionID string) (*client.SubmissionStatusResponse, error) {
	spinner := ui.NewSpinner("评测中")
	defer spinner.Stop()
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		status, err := c.GetSubmissionStatus(submissionID)
		if err != nil {
			return nil, fmt.Errorf("查询状态失败: %w", err)
		}
		if client.IsTerminalStatus(status.Status) {
			return status, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, errors.New("评测超时，请稍后在网页查看结果")
}

func renderResult(result *client.SubmissionStatusResponse, skipLogs bool) error {
	fmt.Println() // blank line before result
	durationStr := ""
	if result.DurationMs != nil {
		durationStr = fmt.Sprintf(" (%.1fs)", float64(*result.DurationMs)/1000)
	}

	switch result.Status {
	case "success":
		ui.Success(fmt.Sprintf("✅ %s「%s」通过！%s", result.StageSlug, result.StageName, durationStr))
		if result.StagePosition > 0 && result.RepoID != "" {
			fmt.Println()
			url := fmt.Sprintf("https://www.bootcraft.cn/courses/%s/stages/%d?repo=%s", result.CourseSlug, result.StagePosition, result.RepoID)
			ui.Info(fmt.Sprintf("👉 前往网页点击「完成本关」解锁下一关：%s", url))
		}
		return nil
	case "failure":
		ui.Error(fmt.Sprintf("❌ %s「%s」未通过%s", result.StageSlug, result.StageName, durationStr))
		if !skipLogs && result.Logs != "" {
			fmt.Println()
			fmt.Println(result.Logs)
		}
		return errors.New("评测未通过")
	case "error":
		ui.Error(fmt.Sprintf("💥 评测出错%s", durationStr))
		if !skipLogs && result.Logs != "" {
			fmt.Println()
			fmt.Println(result.Logs)
		}
		return errors.New("评测出错")
	case "timeout":
		ui.Warn("⏰ 评测超时，请稍后在网页查看结果")
		return errors.New("评测超时")
	default:
		return fmt.Errorf("未知评测状态: %s", result.Status)
	}
}

func findBootcraftConfig() (*bootcraftMeta, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	for {
		configPath := filepath.Join(dir, "bootcraft.yml")
		data, err := os.ReadFile(configPath)
		if err == nil {
			var meta bootcraftMeta
			if err := yaml.Unmarshal(data, &meta); err != nil {
				return nil, "", fmt.Errorf("解析 bootcraft.yml 失败: %w", err)
			}
			if meta.Course == "" || meta.Language == "" {
				return nil, "", errors.New("bootcraft.yml 缺少 course 或 language 字段")
			}
			return &meta, dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, "", errors.New("找不到 bootcraft.yml，请在课程目录中运行此命令")
}

func runGit(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	kb := float64(b) / float64(unit)
	if kb < 1024 {
		return fmt.Sprintf("%.1fKB", kb)
	}
	mb := kb / 1024
	return fmt.Sprintf("%.1fMB", mb)
}
