package commands

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/bootcraft-cn/cli/internal/client"
	"github.com/bootcraft-cn/cli/internal/config"
	"github.com/bootcraft-cn/cli/internal/ui"
)

func LoginCommand(args []string) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	manualToken := flags.String("token", "", "直接写入 API Token，跳过浏览器授权")
	apiURL := flags.String("api-url", "", "API 地址（内部测试用）")
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	baseURL := cfg.GetAPIURL(*apiURL)

	if *manualToken != "" {
		return saveAndVerifyToken(cfg, baseURL, *manualToken)
	}
	return browserLogin(cfg, baseURL)
}

func saveAndVerifyToken(cfg *config.Config, baseURL, token string) error {
	c := client.New(baseURL, token)
	me, err := c.GetMe()
	if err != nil {
		return fmt.Errorf("token 无效或已撤销: %w", err)
	}

	cfg.Token = token
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	ui.Success(fmt.Sprintf("✅ 登录成功！欢迎 %s", me.Username))
	return nil
}

func browserLogin(cfg *config.Config, baseURL string) error {
	c := client.New(baseURL, "")

	resp, err := c.InitCLIAuth()
	if err != nil {
		return fmt.Errorf("初始化认证失败: %w", err)
	}

	ui.Println("正在打开浏览器进行授权...")
	ui.Println(resp.AuthURL)
	openBrowser(resp.AuthURL)

	ui.Print("等待授权（5 分钟内有效）")
	token, username, err := pollForToken(c, resp.Code, resp.ExpiresIn)
	fmt.Println() // newline after dots
	if err != nil {
		return err
	}

	cfg.Token = token
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	ui.Success(fmt.Sprintf("✅ 登录成功！欢迎 %s", username))
	return nil
}

func pollForToken(c *client.Client, code string, expiresIn int) (token, username string, err error) {
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	for time.Now().Before(deadline) {
		result, err := c.GetCLIAuthToken(code)
		if err != nil {
			return "", "", fmt.Errorf("轮询认证状态失败: %w", err)
		}

		switch result.Status {
		case "success":
			return result.Token, result.Username, nil
		case "expired":
			return "", "", errors.New("授权超时，请重新运行 bootcraft login")
		}

		time.Sleep(1 * time.Second)
		ui.Print(".")
	}
	return "", "", errors.New("等待超时，请重新运行 bootcraft login")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
