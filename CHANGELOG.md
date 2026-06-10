# Changelog

All notable changes to MindX will be documented in this file.

## [Unreleased]

### Security
- **API Key 安全存储重构**: 全面修复 API 密钥管理流程中的安全漏洞，严格遵循 Provider APIKey 五条规则：
  - Daemon JSON-RPC (`provider.update` / `model.update`) 不再将用户输入的密钥值明文写入 YAML 配置文件，改为先从环境变量解析实际值后存入 CredentialStore
  - TUI Setup 向导不再修改 `providers.yml` 中 Provider 的 `api_key` 字段（规则3）
  - TUI Client 连接界面不再直接赋值 `Provider.APIKey`，统一通过 CredentialStore 存储密钥
  - App 构造 Runtime 时优先以 `model.provider` 为键从 CredentialStore 读取 APIKey（规则5）
- **Docker 镜像标签修复**: Docker Hub 镜像标签改用 `DOCKER_USER` 命名空间，避免命名冲突

### Features
- **Graph Database & KV Store**: 新增图数据库和 KV 存储功能，包含完整的 RPC 服务端实现、初始化/关闭逻辑、以及 Python 客户端脚本
- **Memory Chunk Query RPC**: 新增内存分块查询 RPC 接口
- **Writer Agent 技能更新**: 为 writer 代理新增 content-factory、copywriting、kg-manager 三项技能配置

### Chore
- 简化 docker-compose 配置，移除冗余的 LLM API 密钥环境变量声明和持久化卷配置

---

## [2.0.7] - 2026-03-12

### Features
- **Internationalization (i18n)**: 新增完整的国际化框架，支持简体中文、繁体中文、英文三种语言；所有硬编码文本已重构为 i18n 多语言键值对；新增语言获取/切换/列出的 RPC 接口

### Changes
- **Dependency Upgrade**: 核心依赖 goreact 升级至 v0.1.5，同步替换前端静态资源文件，更新 mermaid 解析器引用路径
- **Frontend Refresh**: 清理过期前端 chunk 和 diagram 文件，调整版本注入逻辑，新增 `server.version` RPC 接口及调度相关接口
- **Agent & Task Optimization**: 优化 agent 创建逻辑与任务分配脚本
- **Configuration Cleanup**: 移除废弃的 `server.yml` 配置文件；README 文档更新，移除旧架构图占位内容
- **Cross-platform Fix**: 修复 Python venv 路径跨平台适配问题
- **Docker Build Pipeline**: Docker 构建从 CI 迁移至本地执行（`make docker-release`），解决 runtime bin/data 文件 .gitignored 导致 CI 构建失败的问题
- **Dockerfile**: 格式化修正并更新维护者邮箱

---

## [2.0.6] - 2026-03-10

### Documentation
- 更新中英文 README、贡献指南 (CONTRIBUTING)，修复 Homebrew token CI 配置
- 添加完整的中英文文档体系和资源文件

## [2.0.5] - 2026-03-08

### Fixes
- 移除 snap 发布流程中的 continue-on-error（凭据已刷新）
- 修复 snap publish 的 continue-on-error 设置和 nfpm v2 脚本路径
- 修正 nfpm 组织名为 goreleaser/nfpm，替换已弃用的 snap publish action
- 修复 snap publish 输入名称和 nfpm 下载 URL
- 添加 snap 元数据字段并修复 Linux 包工具安装

### Chores
- 将本地文件添加到 gitignore
- 移除意外跟踪的本地文件
- 修复构建脚本中的 mindr → mindx 拼写错误
