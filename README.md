# bootcraft-cli

Bootcraft 平台命令行工具，用于提交课程代码并获取实时评测结果。

## 安装

**从源码构建：**

```bash
git clone https://github.com/bootcraft-cn/bootcraft-cli.git
cd cli
make install
```

> 需要 Go 1.22+。`make install` 会将 `bootcraft` 二进制安装到 `$GOPATH/bin`，请确保该目录在 `$PATH` 中。

## 快速上手

### 1. 登录

```bash
# 浏览器授权登录（推荐）
bootcraft login

# 或使用 Token 登录
bootcraft login --token bc_xxx
```

### 2. 进入课程目录

课程目录中需要有 `bootcraft.yml` 文件：

```yaml
course: tinydsa
language: python
```

### 3. 提交代码

```bash
bootcraft submit
```

提交后 CLI 会实时输出评测日志，评测通过时自动解锁下一关。

## 命令参考

### `bootcraft login`

| 选项              | 说明                                |
| ----------------- | ----------------------------------- |
| `--token <token>` | 使用 API Token 登录，跳过浏览器授权 |

### `bootcraft submit`

| 选项              | 说明                   |
| ----------------- | ---------------------- |
| `--stage <slug>`  | 指定评测关卡           |
| `--dry-run`       | 仅预览打包文件，不上传 |
| `--force`         | 跳过未提交变更确认     |
| `--message <msg>` | 自定义提交备注         |

### `bootcraft version`

显示版本号和构建信息。

## 文件排除规则

打包时会自动排除以下内容：

- `.git/`、`node_modules/`、`__pycache__/`、`.venv/`、`target/` 等常见构建目录
- `.gitignore` 中列出的文件
- `.bootcraftignore` 中列出的文件（可选，格式同 `.gitignore`）

`bootcraft.yml` 始终包含在提交中。使用 `--dry-run` 可预览最终打包文件列表。

## 环境变量

| 变量              | 说明                                   |
| ----------------- | -------------------------------------- |
| `BOOTCRAFT_TOKEN` | API Token，优先于配置文件（适用于 CI） |
| `NO_COLOR`        | 设置后禁用彩色输出                     |

## CI 集成

```bash
export BOOTCRAFT_TOKEN=bc_xxx
bootcraft submit --force
```

评测通过退出码为 `0`，失败为 `1`，可直接用于 CI 流水线判断。

## 配置文件

登录后凭证保存在 `~/.bootcraft/config.yml`（权限 `0600`）。
