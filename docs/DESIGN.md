# bootcraft-cli — 技术设计文档

## 项目结构

参考 `codecrafters-io/cli` 的目录布局：

```
bootcraft-cli/
├── cmd/
│   └── bootcraft/
│       └── main.go             # 入口，flag 解析，subcommand 路由
├── internal/
│   ├── commands/
│   │   ├── login.go            # bootcraft login
│   │   ├── submit.go           # bootcraft submit
│   │   └── version.go          # bootcraft version
│   ├── client/
│   │   ├── client.go           # HTTP client，Bearer token，超时配置
│   │   ├── cli_auth.go         # POST /v1/cli-auth/init, GET /v1/cli-auth/token
│   │   ├── submit.go           # POST /v1/cli/submit (multipart)
│   │   ├── submission.go       # GET /v1/cli/submissions/{id}（轮询状态）
│   │   └── trigger_token.go    # POST /v1/submissions/{id}/trigger-token（SSE 令牌）
│   ├── config/
│   │   └── config.go           # 读写 ~/.bootcraft/config.yml
│   ├── archive/
│   │   └── pack.go             # 目录 → tar.gz，集成 .gitignore/.bootcraftignore
│   └── ui/
│       └── printer.go          # 彩色终端输出封装（基于 fatih/color）
├── docs/
├── .goreleaser.yml
├── Makefile
├── go.mod
└── install.sh
```

---

## 一、入口与命令路由

### `cmd/bootcraft/main.go`

不使用 Cobra（保持轻量，与 codecrafters CLI 风格一致），手动解析 subcommand：

```go
func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }

    switch os.Args[1] {
    case "login":
        err = commands.LoginCommand(os.Args[2:])
    case "submit":
        err = commands.SubmitCommand(os.Args[2:])
    case "version", "--version", "-v":
        commands.VersionCommand()
    default:
        printUsage()
        os.Exit(1)
    }
}
```

---

## 二、配置管理

### 配置文件：`~/.bootcraft/config.yml`

```yaml
token: bc_a3f8d2c1e9b04f7a2d5c8e1f3a6b9d2e7c4a1f8b3e6d9c2a5f0e7b4d1c8a3f6
api_url: https://api.bootcraft.cn # 可选，默认值即此
```

### Token 读取优先级

```
BOOTCRAFT_TOKEN 环境变量  >  config.yml token  >  无（提示未登录）
```

### `internal/config/config.go`

```go
type Config struct {
    Token  string `yaml:"token"`
    APIURL string `yaml:"api_url"`
}

const defaultAPIURL = "https://api.bootcraft.cn"
const configDir  = "~/.bootcraft"
const configFile = "~/.bootcraft/config.yml"

func Load() (*Config, error)         // 读取，不存在则返回空 Config
func (c *Config) Save() error        // 写入，创建目录，chmod 0600
func (c *Config) GetToken() string   // 先查 BOOTCRAFT_TOKEN 环境变量
func (c *Config) GetAPIURL() string  // 先查 --api-url flag，再看 config，再用默认值
```

---

## 三、认证流程 `bootcraft login`

### 时序

```
CLI                          API                         Browser
 │                            │                               │
 │  POST /v1/cli-auth/init    │                               │
 │ ─────────────────────────► │                               │
 │  { code, authUrl, expiresIn: 300 }                        │
 │ ◄───────────────────────── │                               │
 │                            │                               │
 │  open(authUrl)             │                               │
 │ ──────────────────────────────────────────────────────────►│
 │                            │                               │
 │  (每 1s) GET /v1/cli-auth/token?code=xxx                  │
 │ ─────────────────────────► │                               │
 │  { status: "pending" }     │         ← 用户未操作          │
 │ ◄───────────────────────── │                               │
 │                            │        用户确认授权            │
 │ ─────────────────────────► │ ◄──────────────────────────── │
 │  { status: "success",      │                               │
 │    token: "bc_xxx",        │                               │
 │    username: "william" }   │                               │
 │ ◄───────────────────────── │                               │
 │                            │                               │
 │  写入 ~/.bootcraft/config.yml                              │
 │  打印"登录成功！"            │                               │
```

### `internal/commands/login.go`

```go
func LoginCommand(args []string) error {
    flags := flag.NewFlagSet("login", flag.ExitOnError)
    manualToken := flags.String("token", "", "直接写入 token，跳过浏览器授权")
    flags.Parse(args)

    if *manualToken != "" {
        // 直接写入 config 并验证
        return saveAndVerifyToken(*manualToken)
    }

    // 1. 获取 code
    resp, err := client.InitCLIAuth()

    // 2. 打开浏览器
    ui.Println("正在打开浏览器进行授权...")
    openBrowser(resp.AuthURL)

    // 3. 轮询
    ui.Print("等待授权（5 分钟内有效）")
    token, username, err := pollForToken(resp.Code, resp.ExpiresIn)

    // 4. 写入 config
    cfg.Token = token
    cfg.Save()

    ui.Success("登录成功！欢迎 " + username)
    return nil
}

func pollForToken(code string, expiresIn int) (token, username string, err error) {
    deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
    for time.Now().Before(deadline) {
        result, err := client.GetCLIAuthToken(code)
        if result.Status == "success" {
            return result.Token, result.Username, nil
        }
        if result.Status == "expired" {
            return "", "", errors.New("授权超时，请重新运行 bootcraft login")
        }
        time.Sleep(1 * time.Second)
        ui.Print(".")
    }
    return "", "", errors.New("等待超时")
}
```

---

## 四、代码提交 `bootcraft submit`

### 完整流程

```
1. 向上查找 bootcraft.yml → 提取 course + language
2. 读取 Token（config / 环境变量）
3. 尝试读取 git commit 信息（非 git 目录时静默跳过）
4. 扫描目录 → 应用 .gitignore + .bootcraftignore → 打包为 tar.gz（内存中）
5. POST /v1/cli/submit（multipart）
6. SSE 实时展示日志 / 降级轮询
7. 渲染结果
```

### `internal/commands/submit.go`

```go
type SubmitFlags struct {
    Stage   string   // --stage s03-crc32（sequential: 可选；freeform: 必须）
    Message string   // --message：自定义提交备注，覆盖 git commit message
    DryRun  bool     // --dry-run
    Force   bool     // --force（跳过未提交变更确认）
    APIURL  string   // --api-url（测试用）
}

func SubmitCommand(args []string) error {
    flags := parseSubmitFlags(args)

    // 1. 查找 bootcraft.yml
    meta, projectDir, err := findBootcraftConfig()

    // 2. 鉴权
    token := cfg.GetToken()
    if token == "" {
        return errors.New("未登录，请先运行: bootcraft login")
    }

    // 3. 读取 git commit 信息（失败时静默跳过，非 git 目录也能提交）
    commitSHA, _ := runGit(projectDir, "rev-parse", "HEAD")
    commitMsg, _ := runGit(projectDir, "log", "-1", "--format=%s")
    // --message 覆盖 git commit message（不影响 commit_sha）
    if flags.Message != "" {
        commitMsg = flags.Message
    }

    // 4. 检查未提交变更（仅在 git 目录且有 commit SHA 时才检查）
    if commitSHA != "" {
        if dirty, _ := runGit(projectDir, "status", "--porcelain"); dirty != "" {
            ui.Warn("检测到未提交变更，提交结果将不关联 git commit 记录。建议先 git commit 后再提交。")
            if !flags.Force && isatty.IsTerminal(os.Stdin.Fd()) {
                if !ui.Confirm("继续提交？[y/N]") {
                    return errors.New("已取消")
                }
            }
            // 清除 git 自动读取的信息；若用户传了 --message 则保留（显式优先）
            commitSHA = ""
            if flags.Message == "" {
                commitMsg = ""
            }
        }
    }

    // 5. 打包
    ui.Printf("📦 打包代码中...")
    buf, fileCount, totalSize, err := archive.Pack(projectDir)
    ui.Printf(" (%d 个文件, %s)\n", fileCount, formatBytes(totalSize))

    // 客户端预检：快速失败，省去无效上传
    // 与服务端限制保持一致（服务端作为安全边界做最终校验）
    const maxFileCount  = 200
    const maxTotalSize  = 8 * 1024 * 1024 // 8 MB 解压后
    const maxCompressed = 2 * 1024 * 1024 // 2 MB 压缩后
    if fileCount > maxFileCount {
        return fmt.Errorf("文件数量超限（%d > %d），请检查 .gitignore / .bootcraftignore 是否正确排除了构建产物", fileCount, maxFileCount)
    }
    if totalSize > maxTotalSize {
        return fmt.Errorf("代码包解压后大小超限（%s > 8MB），请排除不必要的文件", formatBytes(totalSize))
    }
    if int64(buf.Len()) > maxCompressed {
        return fmt.Errorf("代码包压缩后大小超限（%s > 2MB），请排除不必要的文件", formatBytes(int64(buf.Len())))
    }

    if flags.DryRun {
        ui.Println("[dry-run] 仅预览，不上传")
        return nil
    }

    // 6. 上传
    ui.Print("🚀 上传到评测服务...")
    submitResp, err := client.Submit(SubmitParams{
        Course:        meta.Course,
        Language:      meta.Language,
        Stage:         flags.Stage,
        Archive:       buf,
        CommitSHA:     commitSHA,   // 空字符串时后端用 submission ID 占位
        CommitMessage: commitMsg,
    })
    ui.Println(" 完成")

    // 5. 评测日志：先尝试 SSE，失败则降级轮询
    ui.Println("⏳ 评测中...")
    result, err := watchSubmission(apiClient, submitResp.SubmissionID)

    // 6. 展示结果
    renderResult(result)
    return nil
}

// watchSubmission 优先使用 trigger.dev SSE 实时展示日志；
// 若 SSE 建连失败（网络不通、token 获取失败），自动降级到轮询。
func watchSubmission(c *client.Client, submissionID string) (*client.SubmissionStatusResponse, error) {
    tokenResp, err := c.GetTriggerToken(submissionID)
    if err != nil {
        ui.Warn("实时日志不可用，切换到轮询模式")
        return pollSubmission(c, submissionID)
    }

    result, streamErr := streamEvalLogs(tokenResp.TriggerRunID, tokenResp.PublicAccessToken)
    if streamErr != nil {
        ui.Warn("SSE 连接中断，切换到轮询模式")
        return pollSubmission(c, submissionID)
    }
    return result, nil
}
```

### `bootcraft.yml` 格式

```yaml
course: tinygit
language: python
```

查找逻辑：从 CWD 开始，逐层向上，直到找到 `bootcraft.yml` 或到达文件系统根目录。

---

## 五、打包模块 `internal/archive/pack.go`

### 排除规则优先级

```
1. 硬编码排除（构建产物、依赖目录、二进制文件等，见下表）
2. .gitignore（当前目录 + git 全局配置）
3. .bootcraftignore（格式与 .gitignore 相同，可追加排除规则）
4. bootcraft.yml 本身必须包含（不被以上规则排除）
```

**硬编码排除列表（覆盖主流语言）：**

| 类别              | 排除项                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------------------ |
| VCS               | `.git/`                                                                                          |
| Python            | `__pycache__/`、`*.pyc`、`*.pyo`、`.venv/`、`venv/`、`*.egg-info/`                               |
| Node.js / JS / TS | `node_modules/`、`dist/`、`.next/`、`.nuxt/`                                                     |
| Java / Kotlin     | `target/`（Maven）、`build/`（Gradle）、`out/`、`*.class`、`*.jar`、`*.war`、`*.ear`、`.gradle/` |
| Go                | `vendor/`                                                                                        |
| Rust              | `target/`（已含于上）                                                                            |
| C / C++           | `*.o`、`*.a`、`*.so`、`*.exe`                                                                    |
| 通用              | `.DS_Store`、`Thumbs.db`、`*.log`                                                                |

> **注**：`.gitignore` 已能覆盖大多数项目中的构建产物。硬编码列表作为安全兜底，适用于未配置 `.gitignore` 的项目。用户可通过 `.bootcraftignore` 追加项目特定排除规则。

### 实现

```go
// Pack 扫描 dir，返回 tar.gz 内存 buffer、文件数、总大小
func Pack(dir string) (buf *bytes.Buffer, fileCount int, totalSize int64, err error) {
    gitIgnore := loadGitIgnore(dir)       // 复用 sabhiram/go-gitignore
    bcIgnore  := loadBCIgnore(dir)

    buf = &bytes.Buffer{}
    gw := gzip.NewWriter(buf)
    tw := tar.NewWriter(gw)

    err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        rel, _ := filepath.Rel(dir, path)

        // 硬编码排除
        if shouldAlwaysExclude(rel) { return filepath.SkipDir or nil }

        // .gitignore + .bootcraftignore
        if gitIgnore.MatchesPath(rel) || bcIgnore.MatchesPath(rel) { return nil }

        // 写入 tar
        ...
    })

    tw.Close(); gw.Close()
    return buf, fileCount, totalSize, nil
}

var alwaysExclude = []string{
    // VCS
    ".git",
    // Python
    "__pycache__", "*.pyc", "*.pyo", ".venv", "venv", "*.egg-info",
    // Node.js / JS / TS
    "node_modules", "dist", ".next", ".nuxt",
    // Java / Kotlin (Maven: target, Gradle: build / out)
    "target", "build", "out", "*.class", "*.jar", "*.war", "*.ear", ".gradle",
    // Go
    "vendor",
    // C / C++
    "*.o", "*.a", "*.so", "*.exe",
    // Common
    ".DS_Store", "Thumbs.db", "*.log",
}
```

---

## 六、HTTP Client `internal/client/`

### `client.go`

```go
type Client struct {
    BaseURL    string
    Token      string
    httpClient *http.Client
}

func New(baseURL, token string) *Client {
    return &Client{
        BaseURL: baseURL,
        Token:   token,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
    if c.Token != "" {
        req.Header.Set("Authorization", "Bearer " + c.Token)
    }
    req.Header.Set("User-Agent", "bootcraft-cli/" + Version)
    return c.httpClient.Do(req)
}
```

### `cli_auth.go`

```go
type InitAuthResponse struct {
    Code      string `json:"code"`
    AuthURL   string `json:"authUrl"`
    ExpiresIn int    `json:"expiresIn"`
}

func (c *Client) InitCLIAuth() (*InitAuthResponse, error)

type PollAuthResponse struct {
    Status   string `json:"status"`   // pending | success | expired
    Token    string `json:"token"`
    Username string `json:"username"`
}

func (c *Client) GetCLIAuthToken(code string) (*PollAuthResponse, error)
```

### `submit.go`

```go
type SubmitParams struct {
    Course        string
    Language      string
    Stage         string     // 可选（sequential 课程）；freeform 课程必须传
    Archive       io.Reader
    CommitSHA     string     // 可选，git rev-parse HEAD
    CommitMessage string     // 可选，git log -1 --format=%s
}

type SubmitResponse struct {
    SubmissionID string `json:"submissionId"`
    Status       string `json:"status"`
    StageSlug    string `json:"stageSlug"`
    StageName    string `json:"stageName"`
}

func (c *Client) Submit(params SubmitParams) (*SubmitResponse, error) {
    // multipart/form-data
    // fields: course, language, stage (sequential: optional; freeform: required)
    //         commit_sha (optional), commit_message (optional)
    // file:   code (tar.gz)
}
```

### `submission.go`

```go
type SubmissionStatusResponse struct {
    ID        string `json:"id"`
    Status    string `json:"status"`       // evaluating | success | failure | error | timeout
    StageName string `json:"stageName"`
    NextStage *struct {
        Name string `json:"name"`
        Slug string `json:"slug"`
    } `json:"nextStage"`
    FailureReason string `json:"failureReason"`
    DurationMs    int    `json:"durationMs"`
}

func (c *Client) GetSubmissionStatus(id string) (*SubmissionStatusResponse, error)
```

### `trigger_token.go`

```go
type TriggerTokenResponse struct {
    PublicAccessToken string `json:"publicAccessToken"`
    TriggerRunID      string `json:"triggerRunId"`
    ExpiresAt         string `json:"expiresAt"`
}

// GetTriggerToken 调 POST /v1/submissions/{id}/trigger-token
// 返回用于直连 trigger.dev SSE 的短期令牌
func (c *Client) GetTriggerToken(submissionID string) (*TriggerTokenResponse, error)
```

---

## 七、评测日志 Watch 策略

### 主流程：SSE 优先，轮询兜底

```
提交成功
    │
    ▼
GET /v1/submissions/{id}/trigger-token
    │
    ├─ 成功 ─► 直连 trigger.dev SSE（eval-logs 流）
    │              │
    │              ├─ 正常结束 ─► 从 /v1/cli/submissions/{id} 取最终状态
    │              │
    │              └─ 连接失败/中断 ─► 降级到轮询
    │
    └─ 失败 ─► 降级到轮询
                   │
                   └─ 每 2s GET /v1/cli/submissions/{id}，最长 120s
```

### SSE 实现 `internal/commands/submit.go`

```go
func streamEvalLogs(runID, accessToken string) (*client.SubmissionStatusResponse, error) {
    url := fmt.Sprintf(
        "https://api.trigger.dev/realtime/v1/runs/%s/streams/eval-logs",
        runID,
    )
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Accept",        "text/event-stream")
    req.Header.Set("Cache-Control", "no-cache")

    // SSE 连接超时设为 5 分钟（覆盖 client 默认 30s）
    sseClient := &http.Client{Timeout: 5 * time.Minute}
    resp, err := sseClient.Do(req)
    if err != nil {
        return nil, err   // 触发降级
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("SSE status %d", resp.StatusCode)  // 触发降级
    }

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            chunk := strings.TrimPrefix(line, "data: ")
            if chunk == "[DONE]" { break }
            fmt.Print(chunk)   // 实时输出日志到终端，保留原始换行
        }
    }
    if err := scanner.Err(); err != nil {
        return nil, err   // 触发降级
    }

    // SSE 结束后取最终提交状态
    return apiClient.GetSubmissionStatus(submissionID)
}
```

### 降级轮询 `pollSubmission`

```go
func pollSubmission(c *client.Client, submissionID string) (*client.SubmissionStatusResponse, error) {
    deadline := time.Now().Add(120 * time.Second)
    for time.Now().Before(deadline) {
        status, err := c.GetSubmissionStatus(submissionID)
        if err != nil { return nil, err }
        if isTerminal(status.Status) { return status, nil }
        fmt.Print(".")
        time.Sleep(2 * time.Second)
    }
    return nil, errors.New("评测超时")
}

var terminalStatuses = map[string]bool{
    "success": true, "failure": true, "error": true, "timeout": true,
}
```

### 降级触发条件

| 场景                                                | 降级                               |
| --------------------------------------------------- | ---------------------------------- |
| `GET /v1/submissions/{id}/trigger-token` 返回非 200 | ✅                                 |
| 网络无法连接 `api.trigger.dev`                      | ✅                                 |
| SSE 响应状态码非 200                                | ✅                                 |
| SSE 连接中途断开（scanner.Err()）                   | ✅                                 |
| CI 环境（无 TTY）                                   | 不主动降级，SSE 仍工作，仅禁用颜色 |

---

## 九、终端输出 `internal/ui/printer.go`

```go
func Success(msg string)     // ✅ 绿色
func Error(msg string)       // ❌ 红色
func Warn(msg string)        // ⚠ 黄色
func Info(msg string)        // 蓝色
func Printf(fmt string, ...) // 正常输出
```

评测结果渲染（参考 codecrafters CLI 风格）：

```
✅ Stage 3「实现 CRC-32」通过！(12.3s)
🎉 下一关已解锁：Stage 4「实现 DEFLATE」
```

```
❌ Stage 3 未通过 (8.1s)

测试输出：
  expected: 0x1c291ca3
  got:      0x00000000
```

---

## 十、版本检查 `bootcraft version`

```go
func checkLatestVersion(currentVersion string) {
    // GET https://api.bootcraft.cn/v1/cli/latest-version
    // 后台 goroutine 执行，command 结束后打印，不阻塞主流程
    go func() {
        latest, err := client.GetLatestVersion()
        if err != nil || !isNewerVersion(latest, currentVersion) { return }
        ui.Warn(fmt.Sprintf("新版本可用 %s，运行 brew upgrade bootcraft-cli 升级", latest))
    }()
}
```

版本号通过 goreleaser ldflags 注入：

```
-X github.com/bootcraft-cn/cli/internal/version.Version={{.Version}}
-X github.com/bootcraft-cn/cli/internal/version.Commit={{.Commit}}
```

---

## 十一、分发配置 `.goreleaser.yml`

```yaml
version: 2
builds:
  - env:
      - CGO_ENABLED=0
    main: ./cmd/bootcraft
    binary: bootcraft
    ldflags: >-
      -s -w
      -X github.com/bootcraft-cn/cli/internal/version.Version={{.Version}}
      -X github.com/bootcraft-cn/cli/internal/version.Commit={{.Commit}}
    goos: [darwin, linux]
    goarch: [amd64, arm64]

archives:
  - name_template: "{{ .Tag }}_{{ .Os }}_{{ .Arch }}"

brews:
  - name: bootcraft-cli
    repository:
      owner: bootcraft-cn
      name: homebrew-tap
    homepage: https://bootcraft.cn
    description: Bootcraft CLI — 在终端提交代码评测
    license: MIT
    install: bin.install "bootcraft"

release:
  github:
    owner: bootcraft-cn
    name: bootcraft-cli
```

---

## 十二、依赖计划

| 库                                 | 用途                                     |
| ---------------------------------- | ---------------------------------------- |
| `github.com/fatih/color`           | 彩色终端输出                             |
| `github.com/sabhiram/go-gitignore` | .gitignore 解析（复用 codecrafters CLI） |
| `gopkg.in/yaml.v3`                 | 读写 config.yml、bootcraft.yml           |
| `golang.org/x/sys`                 | 跨平台文件权限（chmod 0600）             |
| `github.com/mattn/go-isatty`       | TTY 检测（判断是否在交互式终端中运行）   |

其余全部使用 Go 标准库（`net/http`、`archive/tar`、`compress/gzip`、`bufio`、`os`、`flag`）。

> SSE 连接实现不依赖任何第三方 SSE 库，仅用 `net/http` + `bufio.Scanner` 解析 `text/event-stream`。
