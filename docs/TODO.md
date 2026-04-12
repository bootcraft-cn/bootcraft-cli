# bootcraft-cli — 实施 TODO

> 语言：Go 1.23+
> 对应后端：`bootcraft-api/docs/cli-submit-api/`（需后端 Phase 2-3 完成后才能联调）
> 文档日期：2026-04-12

---

## 任务状态说明

- `[ ]` 未开始
- `[-]` 进行中
- `[x]` 已完成

---

## Phase 1：项目初始化（0.5 天）

### 工程骨架

- [x] 创建 Go 模块：`go mod init github.com/bootcraft-cn/cli`
- [x] 创建目录结构：
  ```
  cmd/bootcraft/
  internal/commands/
  internal/client/
  internal/config/
  internal/archive/
  internal/ui/
  internal/version/
  docs/
  ```
- [x] 添加依赖：
  - `github.com/fatih/color`
  - `github.com/sabhiram/go-gitignore`
  - `gopkg.in/yaml.v3`
- [x] 创建 `cmd/bootcraft/main.go`：subcommand 路由（login / submit / version）、`--help` 输出
- [x] 创建 `internal/version/version.go`：`Version`、`Commit` 变量（ldflags 注入）

### 工程配置

- [x] 创建 `Makefile`：`build`、`test`、`lint`、`install` 目标
- [ ] 创建 `.goreleaser.yml`：darwin/linux × amd64/arm64，Homebrew tap 配置
- [ ] 创建 `install.sh`：curl 一键安装脚本，检测 OS/ARCH，下载对应包
- [ ] 创建 `.github/workflows/test.yml`：PR 触发 `go test ./...`
- [ ] 创建 `.github/workflows/release.yml`：tag 触发 goreleaser

### 验收

- [x] `go build ./cmd/bootcraft` 成功，运行 `./bootcraft --help` 输出使用说明
- [x] `./bootcraft version` 输出版本号

---

## Phase 2：配置与 HTTP Client（0.5 天）

### `internal/config/config.go`

- [x] 定义 `Config` struct（`Token`、`APIURL`）
- [x] `Load()`：读取 `~/.bootcraft/config.yml`，不存在时返回默认配置
- [x] `Save()`：写入，创建 `~/.bootcraft/` 目录，文件权限 `0600`
- [x] `GetToken()`：先查 `BOOTCRAFT_TOKEN` 环境变量，再查 config
- [x] `GetAPIURL()`：先查命令行 `--api-url` flag，再查 config，再用 `https://api.bootcraft.cn`

### `internal/ui/printer.go`

- [x] `Success(msg)`、`Error(msg)`、`Warn(msg)`、`Info(msg)`、`Printf(fmt, ...)`
- [x] 检测非 TTY（CI 环境）时禁用颜色（`os.Getenv("NO_COLOR")` 或 `isatty` 检测）

### `internal/client/client.go`

- [x] 定义 `Client` struct：`BaseURL`、`Token`、`http.Client`（Timeout 30s）
- [x] `do(req)` 方法：注入 `Authorization: Bearer`、`User-Agent: bootcraft-cli/<version>`
- [x] 统一错误解析：读取 API 返回 `{ error: { code, message } }`，映射为 Go error

### 验收

- [ ] `bootcraft login --token bc_fake` 写入 config，`cat ~/.bootcraft/config.yml` 可见
- [ ] Config 文件权限为 `0600`

---

## Phase 3：login 命令（1 天）

### `internal/client/cli_auth.go`

- [x] `InitCLIAuth(deviceName string) (*InitAuthResponse, error)`
  - `POST /v1/cli-auth/init`，body `{ "deviceName": "..." }`
  - 返回 `{ code, authUrl, expiresIn }`
- [x] `GetCLIAuthToken(code string) (*PollAuthResponse, error)`
  - `GET /v1/cli-auth/token?code=xxx`
  - 返回 `{ status, token, username }`

### `internal/commands/login.go`

- [x] 解析 `--token` flag
- [x] `--token` 模式：调 `GET /v1/me` 验证 token 有效 → 写入 config → 打印欢迎语
- [x] 浏览器模式：
  - [x] 调 `InitCLIAuth`，打印 authUrl（**打印给用户**，以防浏览器未能自动打开）
  - [x] `openBrowser(url)`：`darwin` 用 `open`，`linux` 用 `xdg-open`，失败时仅打印 URL 不报错
  - [x] `pollForToken`：每 1s 一次，超时提示"授权超时，请重新运行 bootcraft login"
  - [x] 成功：写入 config，打印"✅ 登录成功！欢迎 \<username\>"

### `internal/client/me.go`（新增）

- [x] `GetMe() (*MeResponse, error)`：`GET /v1/me`，验证 token 有效性

### 验收

- [ ] `bootcraft login --token bc_valid` → 登录成功，config 有 token
- [ ] `bootcraft login --token bc_invalid` → 打印"Token 无效或已撤销"
- [ ] `bootcraft login` → 浏览器打开 → 用户授权 → "登录成功"（需后端 Phase 2 完成）
- [ ] `bootcraft login` 授权超时 → 打印提示，退出码非 0

---

## Phase 4：打包模块（0.5 天）

### `internal/archive/pack.go`

- [x] `Pack(dir string) (buf *bytes.Buffer, fileCount int, totalSize int64, err error)`
- [x] 硬编码排除列表（`.git`、`__pycache__`、`node_modules`、`.venv`、`target`、`*.pyc` 等）
- [x] 集成 `.gitignore`（复用 `sabhiram/go-gitignore`，同时加载项目级和全局 git 配置）
- [x] 支持 `.bootcraftignore`（同 .gitignore 格式，文件不存在时跳过）
- [x] `bootcraft.yml` 强制包含（不受排除规则影响）
- [x] 路径安全检查：tar 中条目路径不含 `../`（防 path traversal）
- [x] 打包完成后返回文件数和总大小（用于终端展示）

### `internal/archive/pack_test.go`

- [x] 测试：正常目录打包 → 解压验证文件内容
- [x] 测试：`.gitignore` 排除生效
- [x] 测试：`.bootcraftignore` 排除生效
- [x] 测试：`bootcraft.yml` 存在于结果中
- [ ] 测试：超大单文件（> 1MB）不被打包工具截断（截断由服务端校验）

### 验收

- [ ] `bootcraft submit --dry-run` 在课程目录中打印文件列表，`bootcraft.yml` 在列表中
- [ ] `.git/` 目录不在列表中

---

## Phase 5：submit 命令（1 天）

### `internal/client/submit.go`

- [x] 定义 `SubmitParams` struct（`Course`、`Language`、`Stage`、`Archive`、`CommitSHA`、`CommitMessage`）
- [x] `Submit(params SubmitParams) (*SubmitResponse, error)`
  - `POST /v1/cli/submit`，multipart form data
  - fields: `course`、`language`、`stage`（可选）、`commit_sha`（可选）、`commit_message`（可选）
  - file field: `code`（tar.gz）
  - 正确设置 `Content-Length` 避免服务端超时

### `internal/client/submission.go`

- [x] `GetSubmissionStatus(id string) (*SubmissionStatusResponse, error)`
  - `GET /v1/cli/submissions/{id}`

### `internal/client/trigger_token.go`（新增）

- [x] `GetTriggerToken(submissionID string) (*TriggerTokenResponse, error)`
  - `POST /v1/submissions/{id}/trigger-token`
  - 返回 `{ publicAccessToken, triggerRunId, expiresAt }`

### `internal/commands/submit.go`

- [x] 解析 flags：`--stage`、`--dry-run`、`--force`、`--api-url`
- [x] 向上查找 `bootcraft.yml`（从 CWD 到根目录）
- [x] 未登录时提示"请先运行 bootcraft login"并退出
- [x] 调 `archive.Pack` → 展示文件数和大小
- [x] 尝试读取 git 信息：`runGit(projectDir, "rev-parse", "HEAD")` 和 `runGit(projectDir, "log", "-1", "--format=%s")`
  - 失败时（非 git 目录）静默跳过，`CommitSHA` 和 `CommitMessage` 传空字符串
- [x] 检查未提交变更：`runGit(projectDir, "status", "--porcelain")`
  - 仅在 `commitSHA != ""` 时执行（确认是 git 目录）
  - 输出非空时打印警告
  - TTY 且未传 `--force`：调 `ui.Confirm("继续提交？[y/N]")`，用户选 N 则 `return error`（exit 1）
  - 非 TTY（CI）或 `--force`：静默继续
  - 无论何种情况，继续时将 `commitSHA` / `commitMsg` 置空
- [x] `--dry-run`：打印文件列表后退出
- [x] 调 `client.Submit(SubmitParams{...})` 上传
- [x] 实现 `watchSubmission`：**SSE 优先，失败自动降级轮询**
  - [x] `streamEvalLogs(runID, accessToken string)` ：
    - 用 `net/http` + `bufio.Scanner` 解析 `text/event-stream`
    - 直连 `https://api.trigger.dev/realtime/v1/runs/{runID}/streams/eval-logs`
    - SSE 客户端 Timeout 设为 5 分钟（覆盖默认 30s）
    - `data: [DONE]` 事件时退出循环
    - 任意错误（连接失败、非 200、scanner.Err）返回 error 触发降级
    - 正常结束后调 `GetSubmissionStatus` 取最终状态
  - [x] `pollSubmission(c *client.Client, submissionID string)`：
    - 每 2s 轮询一次，最长 120s
    - 每次打印一个 `.`
  - [x] `GetTriggerToken` 失败时打印提示"实时日志不可用，切换到轮询模式"
  - [x] SSE 连接中断时打印提示"SSE 连接中断，切换到轮询模式"
- [x] 渲染结果：
  - 通过：`✅ Stage N「名称」通过！(Xs)\n🎉 下一关已解锁：...`
  - 失败：`❌ Stage N 未通过 (Xs)\n\n失败原因：...`
  - 超时：`⏰ 评测超时，请稍后在网页查看结果`
  - 网络错误：`🔌 网络错误，请检查连接后重试`
- [x] 结果通过时退出码 0，失败/错误时退出码 1（便于 CI 集成）

### 版本检查集成

- [ ] 在 `main()` 中以后台 goroutine 启动版本检查，command 正常结束后打印提示

### 验收

- [ ] 在真实课程目录执行 `bootcraft submit` → **SSE 模式**：终端实时输出 tester 日志, 评测通过
- [ ] **降级模式验证**：模拟 trigger.dev 不可达（封禁域名）→ 自动切轮询 → 正常获得结果
- [ ] 无 `bootcraft.yml` 的目录执行 → 提示"找不到 bootcraft.yml"
- [ ] 未登录 → 提示"请先登录"
- [ ] 模拟 5MB+ 大文件 → API 返回 413 → CLI 提示"代码包过大（超过 5MB）"
- [ ] 评测失败 → 退出码 1，CI 中可用 `if bootcraft submit; then ...`

---

## Phase 6：发布（0.5 天）

- [ ] 创建 `bootcraft-cn/homebrew-tap` GitHub 仓库（`.github/workflows/` 和 `Formula/`）
- [ ] 配置 goreleaser GitHub token secret（`GORELEASER_TOKEN`、`HOMEBREW_TAP_GITHUB_TOKEN`）
- [ ] 打 `v0.1.0` tag → 触发 release workflow → 验证 GitHub Releases 页有各平台产物
- [ ] 验证 `brew install bootcraft-cn/tap/bootcraft-cli` 可安装
- [ ] 验证 `curl -fsSL https://bootcraft.cn/install.sh | sh` 可安装（需挂载 `install.sh`）
- [ ] README.md 写安装说明和快速上手

---

## 回归测试 Checklist

- [ ] macOS arm64：`bootcraft login` + `bootcraft submit` SSE 模式全流程
- [ ] Linux amd64（Docker 容器）：SSE 模式全流程
- [ ] 模拟 trigger.dev 不可达：自动降级到轮询并成功获得结果
- [ ] `BOOTCRAFT_TOKEN` 环境变量优先于 config.yml
- [ ] CI 模拟（无 TTY）：颜色禁用，仅纯文本输出
- [ ] 超大代码包（> 5MB）→ 明确错误提示
- [ ] 评测失败 → 退出码 1
- [ ] `bootcraft version` 输出正确版本号和 commit
- [ ] Config 文件权限 `0600`（非 `0644`）

---

## 估时汇总

| Phase    | 工作内容                  | 天数     |
| -------- | ------------------------- | -------- |
| 1        | 项目初始化、工程配置      | 0.5      |
| 2        | 配置管理、HTTP Client、UI | 0.5      |
| 3        | login 命令                | 1        |
| 4        | 打包模块                  | 0.5      |
| 5        | submit 命令               | 1        |
| 6        | 发布                      | 0.5      |
| **合计** |                           | **4 天** |

> 需要后端 `bootcraft-api` Phase 2（认证端点）完成后才能联调 login；Phase 3（Submit 端点）完成后才能联调 submit。
