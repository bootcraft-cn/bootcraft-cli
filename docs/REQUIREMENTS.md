# bootcraft-cli — 需求文档

## 背景

Bootcraft 平台原有提交评测流程依赖 GitHub App：用户在 Web 端手动触发，由 trigger.dev 通过 GitHub App Installation Token clone 仓库后评测。

该方案存在以下问题：

| 问题                | 说明                                                   |
| ------------------- | ------------------------------------------------------ |
| 体验割裂            | 用户必须回到浏览器手动点击"提交"，无法在终端完成全流程 |
| GitHub App 权限摩擦 | "仅选择部分仓库"导致新课程仓库需手动追加授权           |
| 无离线工作流支持    | 无 CI/CD 集成路径，GitHub Actions 场景无法使用         |

**MVP 阶段决策：GitHub App 流程完全废弃，使用 CLI 主动推送代码替代。**

## 目标

- 提供 `bootcraft login` 命令，让用户在终端完成平台认证
- 提供 `bootcraft submit` 命令，在终端一键打包并提交代码评测
- 支持从 CI（GitHub Actions 等）以无交互方式提交
- 单二进制分发，支持 macOS / Linux，通过 Homebrew 安装

---

## 功能需求

### R01：CLI 认证 — Web Authorization Flow

```
$ bootcraft login

> 正在打开浏览器进行授权...
> https://bootcraft.cn/cli-auth?code=bc_tmp_xxxx
> 等待授权（5 分钟内有效）...
✅ 登录成功！欢迎 william-yangbo
```

| 编号  | 需求                                                                               |
| ----- | ---------------------------------------------------------------------------------- |
| R01-1 | CLI 调 `POST /v1/cli-auth/init` 获取一次性 `code` 和 `authUrl`                     |
| R01-2 | CLI 自动调用系统默认浏览器打开 `authUrl`                                           |
| R01-3 | CLI 以 1 秒间隔轮询 `GET /v1/cli-auth/token?code=xxx`，直到 `success` 或 `expired` |
| R01-4 | 获取 token 后写入 `~/.bootcraft/config.yml`（`token` 字段）                        |
| R01-5 | `code` 5 分钟过期，过期时 CLI 提示"授权超时，请重新运行 bootcraft login"           |
| R01-6 | 支持 `bootcraft login --token <bc_xxx>` 手动写入 token（CI 场景）                  |
| R01-7 | 支持 `BOOTCRAFT_TOKEN` 环境变量，优先级高于配置文件（CI 场景）                     |

### R02：代码提交 — CLI Submit

```
$ bootcraft submit

📦 打包代码中... (48 个文件, 52KB)
🚀 上传到评测服务...
⏳ 评测中...

✅ Stage 3「实现 CRC-32」通过！(12.3s)
🎉 下一关已解锁：Stage 4「实现 DEFLATE」
```

| 编号   | 需求                                                                                                                                                                                                                                            |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R02-1  | CLI 向上查找最近的 `bootcraft.yml`，提取 `course` 和 `language`                                                                                                                                                                                 |
| R02-2  | 将当前目录打包为 `.tar.gz`，遵循 `.gitignore` + `.bootcraftignore` 排除规则                                                                                                                                                                     |
| R02-3  | 尝试读取当前 git 仓库的 commit 信息（`commit_sha`、`commit_message`），随代码包一起上传                                                                                                                                                         |
| R02-4  | 若检测到 git 工作区有未提交变更（`git status --porcelain` 非空）：<br>• **TTY 环境**：打印警告，提示 `继续提交？[y/N]`，用户选 N 则退出（exit code 1）<br>• **非 TTY 环境（CI）**：打印警告后自动继续<br>• **`--force` flag**：跳过提示直接继续 |
| R02-5  | 警告内容：`⚠ 检测到未提交变更，提交结果将不关联 git commit 记录。建议先 git commit 后再提交。`                                                                                                                                                  |
| R02-6  | 调用 `POST /v1/cli/submit`（multipart）上传代码包及 commit 信息                                                                                                                                                                                 |
| R02-7  | 实时展示评测日志（trigger.dev SSE），SSE 不可用时自动降级到轮询                                                                                                                                                                                 |
| R02-8  | 评测结束后展示结果：通过 / 失败（含失败原因摘要）                                                                                                                                                                                               |
| R02-9  | 支持 `--stage <slug>` 指定评测关卡：sequential 课程可选（不传则使用当前进度关卡）；freeform 课程必须传（无固定进度，API 无法推断 `current_stage_id`）                                                                                           |
| R02-10 | 支持 `--dry-run`：仅打包并展示文件列表，不上传                                                                                                                                                                                                  |
| R02-11 | 非 git 目录时跳过 commit 信息和工作区检查，提交仍可正常进行；后端用 submission ID 作占位                                                                                                                                                        |
| R02-12 | 支持 `--force`：跳过未提交变更确认，直接提交（适用于脚本/CI 明确指定场景）                                                                                                                                                                      |
| R02-13 | 支持 `--message <text>`：自定义提交备注，覆盖从 git 自动读取的 `commit_message`；适用于：有未提交变更时补充说明、非 git 项目、CI 中传入语义化描述；不影响 `commit_sha`（SHA 仍照常读取或因脏工作区清除）                                        |

### R03：版本管理

| 编号  | 需求                                                                  |
| ----- | --------------------------------------------------------------------- |
| R03-1 | `bootcraft version` 输出当前版本号和 commit hash                      |
| R03-2 | 每次执行命令时，后台检查是否有新版本；若有，在命令结束后提示          |
| R03-3 | 提示格式：`⚠ 新版本可用 v1.2.3，运行 brew upgrade bootcraft-cli 升级` |

### R04：配置文件

| 编号  | 需求                                              |
| ----- | ------------------------------------------------- |
| R04-1 | 配置路径：`~/.bootcraft/config.yml`               |
| R04-2 | 文件权限：`0600`（仅当前用户可读写）              |
| R04-3 | 支持 `--api-url` flag 覆盖 API 地址（内部测试用） |

### R05：安全要求

| 编号  | 需求                                                                               |
| ----- | ---------------------------------------------------------------------------------- |
| R05-1 | Token 存储在配置文件中，不打印到终端（执行 `bootcraft login` 后不回显原始 token）  |
| R05-2 | 打包时排除 `.env`、`*.key`、`*.pem` 等敏感文件（默认 `.bootcraftignore` 内置规则） |
| R05-3 | HTTPS 通信，证书验证不可绕过                                                       |

### R06：分发

| 编号  | 需求                                                                       |
| ----- | -------------------------------------------------------------------------- |
| R06-1 | 支持平台：macOS (arm64/amd64)、Linux (amd64/arm64)                         |
| R06-2 | 通过 Homebrew 安装：`brew install bootcraft-cn/tap/bootcraft-cli`          |
| R06-3 | 提供 curl 一键安装脚本：`curl -fsSL https://bootcraft.cn/install.sh \| sh` |
| R06-4 | GitHub Releases 提供各平台压缩包下载                                       |

---

## 非功能需求

| 需求         | 目标                                 |
| ------------ | ------------------------------------ |
| 冷启动时间   | 任意命令 < 100ms                     |
| 打包速度     | 万文件项目 < 3s                      |
| 二进制大小   | < 20MB（单平台）                     |
| 无外部运行时 | 静态二进制，用户不需要安装任何运行时 |

---

## 超出范围（MVP 不做）

- `bootcraft test`：本地运行 tester（需要 tester 二进制本地化）
- `bootcraft init`：初始化课程模板仓库
- `bootcraft status`：查看当前进度
- Windows 支持
