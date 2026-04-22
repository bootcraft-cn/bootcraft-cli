# GitHub Actions 自动提交 — 设计方案

学员克隆官方 starter 仓库做题,**每次 `git push` 自动触发评测**,无需手动跑 `bootcraft submit`。

本文档为产品 + 工程设计稿,落地后会拆分为:starter 内置 workflow、平台 token 管理、可选官方 Action。

---

## 目录

- [1. 目标与范围](#1-目标与范围)
- [2. 用户旅程](#2-用户旅程)
- [3. 架构概览](#3-架构概览)
- [4. starter 内置 workflow(MVP 主方案)](#4-starter-内置-workflow-mvp-主方案)
- [5. 平台侧需要的能力](#5-平台侧需要的能力)
- [6. bootcraft-cli 需要的改进](#6-bootcraft-cli-需要的改进)
- [7. 进阶:官方 GitHub Action 包装](#7-进阶官方-github-action-包装)
- [8. 与作者侧 docker tester CI 的关系](#8-与作者侧-docker-tester-ci-的关系)
- [9. 落地路线图](#9-落地路线图)
- [10. 未决问题](#10-未决问题)

---

## 1. 目标与范围

### 1.1 目标

让学员在 GitHub 上的提交可以**自动**进入 Bootcraft 评测系统,效果等价于在本地跑 `bootcraft submit`:

- ✅ 创建 submission 记录
- ✅ 推进 `repositories.current_stage_id`
- ✅ 写入 commit_sha / commit_message 元数据
- ✅ 学员可在 Actions 日志看到评测结果

### 1.2 非目标

- ❌ 取代本地 `bootcraft submit`(本地仍是主路径,本方案是补充)
- ❌ 取代作者侧 `solution/.github/workflows/test.yml` 的 docker self-test
- ❌ 在 GitHub Runner 内本地跑 tester(评测仍在 trigger.dev,与 CLI 一致)

### 1.3 适用范围

- 所有官方 starter 仓库(`tinygit-{go,python,java}-starter` 等)
- 学员 fork 后的个人仓库
- 不依赖学员是否使用 GitHub(不用就走本地 CLI,workflow 不影响)

---

## 2. 用户旅程

### 2.1 首次使用(一次性配置,2 步)

```
学员
  │
  ├─ 1. 登录 bootcraft.cn → 头像菜单 → "生成 CLI Token" → 复制
  │
  └─ 2. GitHub 仓库 → Settings → Secrets → New
            Name:  BOOTCRAFT_TOKEN
            Value: 粘贴
```

完成。

### 2.2 日常使用

```
学员
  ├─ 改代码
  ├─ git commit -m "fix: read-blob handle short hash"
  └─ git push
        │
        ▼
   GitHub Actions 自动触发
        │
        ├─ 安装 bootcraft CLI
        ├─ bootcraft submit --force
        │     ├─ 打包当前目录(应用 .gitignore)
        │     ├─ POST /v1/cli/submit
        │     └─ SSE 实时显示评测日志
        │
        ▼
   退出码 0 → ✅ 绿色徽章 / 退出码 1 → ❌ 红色失败
```

### 2.3 不想用?

- **不配置 secret** → workflow 自动跳过,不会失败,不打扰学员
- **想完全移除** → 删除 `.github/workflows/bootcraft-submit.yml`

---

## 3. 架构概览

```
学员仓库 (fork 自 starter)
├── bootcraft.yml                          ← 平台已要求,提供 course/language
├── .github/
│   └── workflows/
│       └── bootcraft-submit.yml           ← 本方案核心
└── ... (代码)

           │ push
           ▼
┌──────────────────────────┐
│  GitHub Actions Runner   │
│   ├─ checkout            │
│   ├─ install bootcraft   │
│   └─ bootcraft submit    │
└──────────┬───────────────┘
           │ HTTPS
           ▼
┌──────────────────────────┐         ┌──────────────────┐
│  bootcraft-api           │ ──────► │  trigger.dev     │
│   POST /v1/cli/submit    │         │   eval-logs SSE  │
└──────────────────────────┘         └────────┬─────────┘
                                              │
                                              ▼
                                     学员 Actions 日志
                                     (彩色评测输出)
```

无新增长连接服务,完全复用现有 CLI + bootcraft-api + trigger.dev 链路。

---

## 4. starter 内置 workflow(MVP 主方案)

### 4.1 文件位置

每个 starter 仓库添加:

```
.github/workflows/bootcraft-submit.yml
```

### 4.2 完整内容

```yaml
name: Submit to Bootcraft

on:
  push:
    branches: [main, master]
  workflow_dispatch:

# 学员快速连续 push 时,只评测最新一次
concurrency:
  group: bootcraft-submit-${{ github.ref }}
  cancel-in-progress: true

jobs:
  submit:
    runs-on: ubuntu-latest
    timeout-minutes: 10

    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 1 # commit_sha + last commit message 已足够

      - name: Skip if BOOTCRAFT_TOKEN secret is missing
        env:
          HAS_TOKEN: ${{ secrets.BOOTCRAFT_TOKEN != '' }}
        run: |
          if [ "$HAS_TOKEN" != "true" ]; then
            echo "::warning::BOOTCRAFT_TOKEN secret 未设置,跳过自动提交。"
            echo "如需启用:仓库 Settings → Secrets → Actions → 添加 BOOTCRAFT_TOKEN"
            echo "BOOTCRAFT_SKIP=1" >> $GITHUB_ENV
          fi

      - name: Install bootcraft CLI
        if: ${{ env.BOOTCRAFT_SKIP != '1' }}
        env:
          # 学员可在 repo Variables 里覆盖固定版本,避免新版破坏
          BOOTCRAFT_CLI_VERSION: ${{ vars.BOOTCRAFT_CLI_VERSION || 'latest' }}
        run: |
          set -e
          if [ "$BOOTCRAFT_CLI_VERSION" = "latest" ]; then
            URL="https://github.com/bootcraft-cn/bootcraft-cli/releases/latest/download/bootcraft-linux-amd64"
          else
            URL="https://github.com/bootcraft-cn/bootcraft-cli/releases/download/${BOOTCRAFT_CLI_VERSION}/bootcraft-linux-amd64"
          fi
          curl -fsSL "$URL" -o /tmp/bootcraft
          chmod +x /tmp/bootcraft
          sudo mv /tmp/bootcraft /usr/local/bin/bootcraft
          bootcraft version

      - name: Submit
        if: ${{ env.BOOTCRAFT_SKIP != '1' }}
        env:
          BOOTCRAFT_TOKEN: ${{ secrets.BOOTCRAFT_TOKEN }}
          NO_COLOR: "1"
        run: |
          bootcraft submit \
            --force \
            --message "${{ github.event.head_commit.message }}"
```

### 4.3 设计理由(逐项说明)

| 决策                                      | 理由                                                   |
| ----------------------------------------- | ------------------------------------------------------ |
| `branches: [main, master]`                | 兼容老仓库默认 master                                  |
| `workflow_dispatch`                       | 允许学员手动重跑(网页一键 retry)                       |
| `concurrency.cancel-in-progress`          | 学员一晚上 push 20 次只评测最新,省 trigger.dev 配额    |
| token 缺失走 warning 而非 fail            | 学员忘配/不想用,Actions 不会一片红打扰                 |
| `--force`                                 | CI 无 TTY,防御性跳过脏工作区 prompt                    |
| `--message` 取 commit message             | bootcraft 网页能看到学员真实描述,而非默认空            |
| `NO_COLOR=1`                              | Actions 日志不带 ANSI 转义,搜索友好                    |
| `fetch-depth: 1`                          | git rev-parse HEAD + git log -1 已够,不浪费 clone 时间 |
| `timeout-minutes: 10`                     | CLI 自身 120s 评测超时 + 安装/checkout 余量            |
| `BOOTCRAFT_CLI_VERSION` 走 repo Variables | 学员可锁版本,避免新版破坏                              |

### 4.4 README 配套段落

每个 starter 的 `README.md` 末尾追加:

````markdown
## 🚀 GitHub Actions 自动评测(可选)

每次 `git push` 自动提交到 Bootcraft,在 Actions 页面查看评测结果。

### 一次性配置

1. 登录 [bootcraft.cn](https://bootcraft.cn) → 头像 → **生成 CLI Token** → 复制
2. GitHub 仓库 → Settings → Secrets and variables → Actions → New repository secret
   - Name: `BOOTCRAFT_TOKEN`
   - Value: 粘贴

完成。下次 push 自动触发评测。

### 锁定 CLI 版本(可选)

新版 CLI 偶尔有 breaking change,生产建议锁版本:

GitHub 仓库 → Settings → Secrets and variables → Actions → **Variables** → New

- Name: `BOOTCRAFT_CLI_VERSION`
- Value: `v0.5.0`(或任意 release tag)

### 不想用?

- 不设 `BOOTCRAFT_TOKEN` → workflow 自动跳过,不会失败
- 或直接删除 `.github/workflows/bootcraft-submit.yml`

### 状态徽章(可选)

在 README 顶部加:

```markdown
![Bootcraft](https://github.com/<USER>/<REPO>/actions/workflows/bootcraft-submit.yml/badge.svg)
```
````

---

## 5. 平台侧需要的能力

为了让上述 workflow 跑通,bootcraft-api / 网页需要补:

### 5.1 必需(MVP 阻塞项)

| 能力                             | 说明                                                                | 优先级  |
| -------------------------------- | ------------------------------------------------------------------- | ------- |
| **网页"生成 CLI Token"按钮**     | 头像菜单 / 设置页提供生成入口,按钮一键复制                          | 🔴 必须 |
| **bootcraft-cli release 二进制** | GitHub Release 提供至少 `bootcraft-linux-amd64`(workflow 直接 curl) | 🔴 必须 |

### 5.2 推荐

| 能力                          | 说明                                                           | 优先级  |
| ----------------------------- | -------------------------------------------------------------- | ------- |
| **Token 列表 + 撤销**         | 学员遗失/换设备/离职团队成员需要能撤销                         | 🟡 推荐 |
| **Token 命名 + 创建时间显示** | 区分"我的笔记本"vs "GitHub Actions"                            | 🟡 推荐 |
| **Submission 详情可分享 URL** | 学员可贴链接到 issue 求助                                      | 🟡 推荐 |
| **多平台二进制矩阵**          | macOS arm64/amd64 + Windows amd64 + Linux arm64,本地用户也受益 | 🟢 可选 |

### 5.3 Token 安全建议

- Token 前缀固定 `bc_`,便于 GitHub secret scanning
- 推荐 90 天过期 + 提醒续期
- 服务端记录 last-used 时间,长期未用自动失效

---

## 6. bootcraft-cli 需要的改进

以下改进可显著提升 CI 体验,**非阻塞**但强烈建议:

### 6.1 高优先级

| 改进                              | 价值                                | 工作量 |
| --------------------------------- | ----------------------------------- | ------ |
| **`--workdir <dir>` 标志**        | monorepo / 学员把 starter 放子目录  | S      |
| **上传重试 + 指数退避**           | GitHub Runner 网络抖动不直接红      | S      |
| **打印 submission_id + 网页 URL** | 学员在 Actions 日志直接点链接看详情 | XS     |

### 6.2 中优先级

| 改进                         | 价值                                          | 工作量 |
| ---------------------------- | --------------------------------------------- | ------ |
| **`--json` 结构化输出**      | CI 后续步骤可解析,例如自动 PR 评论            | M      |
| **`--no-watch` / `--async`** | 不等评测结果立即退出,适合不关心实时日志的 CI  | XS     |
| **trigger.dev URL 解耦**     | 由服务端在 trigger-token 响应里返回 streamUrl | S      |

### 6.3 低优先级

| 改进              | 价值                              | 工作量                     |
| ----------------- | --------------------------------- | -------------------------- |
| **多平台二进制**  | linux/macos/windows × amd64/arm64 | M(主要是 release pipeline) |
| **校验 checksum** | workflow 可校验下载完整性         | S                          |

---

## 7. 进阶:官方 GitHub Action 包装

把上面的 workflow 包装成 `bootcraft-cn/submit-action@v1`,学员配置降到 3 行:

### 7.1 学员侧

```yaml
# .github/workflows/bootcraft-submit.yml
name: Submit
on: [push, workflow_dispatch]
jobs:
  submit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: bootcraft-cn/submit-action@v1
        with:
          token: ${{ secrets.BOOTCRAFT_TOKEN }}
          # 可选
          # cli-version: v0.5.0
          # workdir: ./solution
```

### 7.2 Action 仓库结构

```
bootcraft-cn/submit-action/
├── action.yml          ← 输入定义 + composite steps
├── README.md
├── LICENSE
└── .github/workflows/
    ├── test.yml        ← 自测
    └── release.yml     ← tag 触发 release-please
```

### 7.3 action.yml 草案

```yaml
name: "Bootcraft Submit"
description: "Submit code to Bootcraft for evaluation"
branding:
  icon: "send"
  color: "blue"

inputs:
  token:
    description: "Bootcraft API token (BOOTCRAFT_TOKEN)"
    required: true
  cli-version:
    description: "bootcraft-cli version to install (default: latest)"
    required: false
    default: "latest"
  workdir:
    description: "Working directory containing bootcraft.yml"
    required: false
    default: "."
  message:
    description: "Submission message (default: head commit message)"
    required: false
    default: ""

runs:
  using: "composite"
  steps:
    - name: Install CLI
      shell: bash
      env:
        VERSION: ${{ inputs.cli-version }}
      run: |
        # ... 同上 install 逻辑 ...

    - name: Submit
      shell: bash
      working-directory: ${{ inputs.workdir }}
      env:
        BOOTCRAFT_TOKEN: ${{ inputs.token }}
        NO_COLOR: "1"
        MSG: ${{ inputs.message != '' && inputs.message || github.event.head_commit.message }}
      run: bootcraft submit --force --message "$MSG"
```

### 7.4 何时做

不阻塞 MVP,**等内置 workflow 跑顺、学员反馈确认价值后再发布 Action**。Action 一旦发布需 SemVer 维护,不可轻量更改。

---

## 8. 与作者侧 docker tester CI 的关系

容易混淆,在此明确区分:

|           | **学员 starter:`bootcraft-submit.yml`** | **作者 solution:`test.yml`**             |
| --------- | --------------------------------------- | ---------------------------------------- |
| 仓库位置  | `tinygit-{go,python,java}-starter/`     | `bootcraft-courses/tinygit/solution/`    |
| 触发条件  | 学员 push 到 main                       | 作者 push 到 `go` 分支 / tester 镜像更新 |
| 评测位置  | **远程** trigger.dev                    | **本地** docker(Runner 内)               |
| 用 CLI?   | ✅ `bootcraft submit`                   | ❌ `docker run tinygit-tester`           |
| 写 DB?    | ✅ 创建 submission,推进 stage           | ❌ 仅自检                                |
| 网页可见? | ✅ 学员主页有记录                       | ❌ 不可见                                |
| 适用人群  | 学员                                    | 课程维护者                               |

**两套 CI 互补,不要合并、不要互相替代。**

---

## 9. 落地路线图

| #   | 任务                                                      | Owner     | 工作量 | 阻塞?       |
| --- | --------------------------------------------------------- | --------- | ------ | ----------- |
| 1   | bootcraft-cli 发 GitHub Release(linux-amd64 起步)         | CLI       | 半天   | 🔴 阻塞所有 |
| 2   | 网页"生成 CLI Token" + 复制按钮                           | api + web | 1 天   | 🔴 阻塞所有 |
| 3   | 把 workflow yml + README 段落加到 3 个 starter            | 课程      | 1 小时 | —           |
| 4   | E2E 验证:fork starter → 加 token → push → 看 Actions 通过 | QA        | 1 小时 | —           |
| 5   | bootcraft-cli 加 `--workdir` + 上传重试                   | CLI       | 1 天   | 🟡 高优先   |
| 6   | bootcraft-cli 多平台 release 矩阵                         | CLI       | 1 天   | 🟢 可后置   |
| 7   | Token 撤销 + 命名管理界面                                 | api + web | 2 天   | 🟢 可后置   |
| 8   | 发布官方 `bootcraft-cn/submit-action@v1`                  | infra     | 2 天   | 🟢 看反馈   |

**关键路径**:任务 1 + 2 完成后即可立刻铺到 starter,学员就能用上。

---

## 10. 未决问题

| #   | 问题                                                                                                    | 待决策                                                                                  |
| --- | ------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| Q1  | starter 是否默认在 `main` 分支带 workflow,还是放 `examples/` 让学员自取?                                | 推荐**默认带**,门槛最低                                                                 |
| Q2  | 学员 fork 后 push 第一次会触发 workflow,但当时 token 未配 → warning 跳过。是否在 README 显眼位置先提醒? | 是,README 顶部就提                                                                      |
| Q3  | 多个 stage 失败时,只显示一个还是全部?当前 CLI 是单 stage 评测                                           | 不变,CLI 行为一致                                                                       |
| Q4  | 是否在 PR 也触发?当前只 push                                                                            | 暂不,PR 评测会重复占用 trigger.dev 配额。可加 `pull_request` 但用 `concurrency` 限同 PR |
| Q5  | Token 是否绑定单个 repository(类似 GitHub fine-grained PAT)?                                            | 后续考虑;MVP 用账号级 token                                                             |
| Q6  | 国内学员从 GitHub 下载 release 二进制慢,是否提供 CDN 镜像?                                              | 看反馈,可后续接腾讯云/阿里云 OSS 镜像                                                   |

---

## 附录 A:相关文档

- 当前 CLI 设计: [../DESIGN.md](../DESIGN.md)
- 当前 CLI 需求: [../REQUIREMENTS.md](../REQUIREMENTS.md)
- 当前 CLI TODO: [../TODO.md](../TODO.md)
- 学员 starter 示例: `bootcraft-courses/tinygit/starter/tinygit-go-starter/`
- 作者 solution self-test 参考: `bootcraft-courses/tinygit/solution/.github/workflows/test.yml`

## 附录 B:文档索引(本目录)

- `README.md` — 本文(总体设计)
- `workflow-template.yml` — 直接可复制到 starter 的 workflow 文件(同 §4.2)
- `readme-snippet.md` — 直接可粘贴到 starter README 的使用段落(同 §4.4)

后续可拆分:

- `action-spec.md` — 官方 Action 详细 API 设计(对应 §7,发布前撰写)
- `token-management.md` — 平台侧 token 管理详细设计(对应 §5,实现前撰写)
