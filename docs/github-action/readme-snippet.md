<!--
  Starter README 配套段落 — 直接复制到 starter README.md 末尾
  文档:bootcraft-cli/docs/github-action/README.md
-->

## 🚀 GitHub Actions 自动评测(可选)

每次 `git push` 自动提交到 Bootcraft,在 Actions 页面查看评测结果。

### 一次性配置(2 步)

1. 登录 [bootcraft.cn](https://bootcraft.cn) → 头像菜单 → **生成 CLI Token** → 复制
2. GitHub 仓库 → Settings → Secrets and variables → Actions → **New repository secret**
   - Name: `BOOTCRAFT_TOKEN`
   - Value: 粘贴上一步的 token

完成 ✅。下次 `git push` 自动触发评测。

### 锁定 CLI 版本(可选)

新版 CLI 偶尔会有 breaking change,生产环境建议锁版本:

GitHub 仓库 → Settings → Secrets and variables → Actions → **Variables** → New

- Name: `BOOTCRAFT_CLI_VERSION`
- Value: `v0.5.0`(或任意 release tag)

不设置则始终使用 `latest`。

### 状态徽章(可选)

在 README 顶部加:

```markdown
![Bootcraft](https://github.com/<你的用户名>/<仓库名>/actions/workflows/bootcraft-submit.yml/badge.svg)
```

### 不想用?

- 不设 `BOOTCRAFT_TOKEN` → workflow 自动跳过,不会失败,不打扰
- 完全移除 → 删除 `.github/workflows/bootcraft-submit.yml`

### 常见问题

**Q: workflow 跳过了,什么原因?**
A: `BOOTCRAFT_TOKEN` secret 未设置。检查仓库 Settings → Secrets。

**Q: 评测失败但本地能通过?**
A: 检查打包是否包含必要文件。CLI 默认排除 `node_modules` / `target` / `__pycache__` 等;若依赖产物在这些目录,需要在 `.bootcraftignore` 里覆盖。

**Q: Token 泄露怎么办?**
A: 登录 bootcraft.cn → 设置 → Tokens → 撤销,重新生成。
