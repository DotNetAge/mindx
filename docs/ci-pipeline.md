# MindX CI/CD 流水线说明

## 概述

项目使用 GitHub Actions 实现持续集成，配置文件位于 `.github/workflows/ci.yml`。流水线在以下事件触发时自动运行：

- 推送到 `main` 分支
- 向 `main` 分支发起 Pull Request

流水线包含两个并行执行的 Job：`backend` 和 `frontend`。

---

## Job 详情

### backend

运行环境：`ubuntu-latest`

| 步骤 | 说明 |
|------|------|
| checkout | 拉取代码 |
| setup-go | 根据 `go.mod` 安装对应 Go 版本 |
| `go vet ./...` | 静态分析，检查常见错误 |
| 准备测试工作区 | 将 `config/` 复制到 `.test/config/`，避免污染生产数据 |
| `go test ./...` | 运行全部后端测试，`MINDX_WORKSPACE` 指向 `.test` 目录 |

### frontend

运行环境：`ubuntu-latest`，工作目录固定为 `dashboard/`

| 步骤 | 说明 |
|------|------|
| checkout | 拉取代码 |
| setup-node | 安装 Node.js 20，启用 npm 缓存 |
| `npm ci` | 安装依赖（基于 lock 文件，确保可复现） |
| `npm run lint` | ESLint 代码规范检查 |
| `npx tsc --noEmit` | TypeScript 类型检查（不生成产物） |
| `npm run test` | Vitest 单元测试 |

---

## 本地复现

在提交前可在本地运行相同的检查：

```bash
# 后端
go vet ./...
cp -r config .test/config
MINDX_WORKSPACE=$(pwd)/.test go test ./...

# 前端
cd dashboard
npm run lint
npx tsc --noEmit
npm run test
```

或使用 Makefile 快捷命令：

```bash
make lint    # go lint
make test    # 后端测试（自动设置 .test 工作区）
make build   # 前后端完整构建
```

---

## 静态分析配置

后端使用 golangci-lint，配置文件 `.golangci.yml` 启用了以下 linter：

| Linter | 作用 |
|--------|------|
| govet | 检查可疑的代码结构 |
| errcheck | 检查未处理的错误返回值 |
| staticcheck | 高级静态分析 |
| unused | 检查未使用的代码 |

---

## 前端测试框架

- 测试运行器：Vitest
- 组件测试：React Testing Library
- DOM 环境：jsdom
- 配置位置：`dashboard/vite.config.ts` 中的 `test` 字段
- 测试初始化：`dashboard/src/test/setup.ts`

测试文件与组件同目录放置，命名规则为 `*.test.tsx`。

---

## 常见问题

**Q: 后端测试为什么需要 `.test` 工作区？**

MindX 运行时会在 `MINDX_WORKSPACE` 下读写配置和数据。测试使用独立的 `.test` 目录，避免影响 `~/.mindx` 中的生产数据。

**Q: 前端测试失败但本地构建正常？**

确认 `package-lock.json` 已提交。CI 使用 `npm ci` 严格按 lock 文件安装，本地 `npm install` 可能引入版本差异。

**Q: 如何添加新的测试？**

后端：在对应包目录下创建 `*_test.go` 文件。
前端：在组件同目录下创建 `*.test.tsx` 文件，参考 `SkillCard.test.tsx` 的写法。
