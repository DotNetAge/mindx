---
name: docker-expert
description: >
  当用户要求"创建 Dockerfile"、"优化 Docker 镜像"、"多阶段构建"、"容器安全"、"Docker Compose"、"docker-compose.yml"、"减小镜像体积"、"Docker 最佳实践"、"容器化这个应用"、"Docker 健康检查"、"dockerignore"、"Docker 网络"、"Docker 卷"、"Docker 密钥"、"容器编排"、"Docker 构建优化"、"非 root 容器"、"Docker 层缓存"，或涉及 Docker 容器化、镜像构建、容器安全加固、Docker Compose 服务编排的任何请求时使用此技能。也适用于审查 Docker 配置、诊断构建失败、排查容器网络问题或搭建带热重载的开发容器。提供全面的 Docker 专业能力，包括多阶段构建优化、安全加固、镜像瘦身和生产级模式。
allowed-tools: bash read_file write_file
metadata:
  name_zh: Docker 专家
  name_zh-tw: Docker 專家
  description_zh: Docker 容器化优化、多阶段构建、安全加固与生产级部署配置
  description_zh-tw: Docker 容器化優化、多階段建置、安全加固與生產級部署配置
---

# Docker 专家技能

将 Docker 配置从可用级别提升到生产级，聚焦优化、安全和可维护性。适用于 Dockerfile、Docker Compose 配置、容器构建或任何容器化任务——从初始搭建到生产加固。

## 何时使用此技能

- 用户想为任何语言或框架创建或改进 Dockerfile
- 用户要求优化现有 Docker 镜像（体积、构建速度、缓存）
- 项目需要从零开始容器化，或迁移到容器
- 用户想为多服务应用搭建 Docker Compose
- 安全扫描发现容器镜像存在漏洞
- 构建缓慢，用户要求加速 Docker 层缓存
- 容器需要安全加固（非 root 用户、密钥管理、权限控制）
- 用户要求审查 Docker 配置的生产就绪状态
- 开发工作流需要带热重载和调试的容器配置
- 用户遇到服务间的 Docker 网络问题
- 用户询问多架构构建或跨平台容器化

## 此技能的功能

1. **分析现有 Docker 配置** — 检测模式、识别反模式、评估生产就绪度
2. **创建生产级 Dockerfile** — 多阶段构建、层优化、安全加固
3. **设计 Docker Compose 配置** — 服务编排、网络、健康检查、密钥管理
4. **优化镜像体积和构建速度** — 从臃肿镜像到精简、缓存友好的构建
5. **加固容器安全** — 非 root 用户、最小攻击面、密钥管理
6. **诊断容器问题** — 构建失败、网络问题、资源限制

## 使用方式

```
为这个 Go API 服务器创建 Dockerfile
```

```
我的 Docker 构建需要 10 分钟。能优化一下加速缓存吗？
```

```
审查我们的 docker-compose.yml 是否达到生产就绪——下周就要部署了
```

```
用多阶段构建将这个 Node.js 应用容器化用于生产
```

```
容器安全扫描发现了 15 个漏洞。帮我加固镜像。
```

```
为这个 React + Express 应用搭建带热重载的开发 Docker Compose 环境
```

## 工作流概览

```
用户描述 Docker 任务
        │
        ▼
  阶段 1：环境检测 — Docker 版本、项目结构、现有配置
        │
        ▼
  阶段 2：问题分析 — 分类：构建、安全、网络、编排、优化
        │
        ▼
  阶段 3：方案设计 — 应用匹配用户技术栈的最佳实践模式
        │
        ▼
  阶段 4：实施 — 编写或修改 Dockerfile、compose 文件、.dockerignore
        │
        ▼
  阶段 5：验证 — 构建测试、安全扫描、运行时验证
```

---

## 操作指南

### 阶段 1：环境检测

**触发条件：** 任何 Docker 相关请求。在给出建议前始终先检测环境。

#### 步骤 1.1 - 检查 Docker 可用性

```bash
docker --version 2>/dev/null || echo "Docker not installed"
docker info --format '{{.ServerVersion}}' 2>/dev/null || echo "Docker daemon not running"
```

| 条件           | 操作                                           |
| -------------- | ---------------------------------------------- |
| Docker 未安装  | 引导用户先安装 Docker Desktop 或 Docker Engine |
| 守护进程未运行 | 请用户先启动 Docker                            |
| Docker 就绪    | 进入步骤 1.2                                   |

#### 步骤 1.2 - 扫描项目结构

查找项目中所有 Docker 相关文件：

```bash
find . -name "Dockerfile*" -type f | head -10
find . -name "*compose*.yml" -o -name "*compose*.yaml" -type f | head -5
find . -name ".dockerignore" -type f | head -3
```

#### 步骤 1.3 - 评估现有状态

如果已有 Dockerfile，检查以下模式：

| 检查项                                   | 重要性                       |
| ---------------------------------------- | ---------------------------- |
| 基础镜像选择（Alpine、slim、distroless） | 影响体积和安全面             |
| 是否有多阶段构建？                       | 如果缺失，这是关键优化机会   |
| 层顺序（依赖在源码之前？）               | 构建缓慢、缓存失效的主要原因 |
| 是否有 USER 指令？                       | 安全基线——绝不应以 root 运行 |
| 是否定义了 HEALTHCHECK？                 | 编排和生产环境必需           |
| EXPOSE 与实际端口                        | 必须匹配应用监听端口         |

#### 步骤 1.4 - 检查运行状态（如适用）

```bash
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | head -10
docker images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}" | head -10
```

### 阶段 2：问题分析

**触发条件：** 环境检测完成后。对用户请求进行分类以应用正确的解决方案。

#### 步骤 2.1 - 请求分类

| 类别                | 典型触发短语                                      | 解决重点                   |
| ------------------- | ------------------------------------------------- | -------------------------- |
| **新建 Dockerfile** | "创建 Dockerfile"、"容器化"、"Dockerize 这个应用" | 从零搭建多阶段构建         |
| **构建优化**        | "构建慢"、"缓存问题"、"每次都全量重建"            | 层顺序、缓存挂载           |
| **镜像体积**        | "镜像太大"、"减小体积"、"精简镜像"                | Distroless、多阶段、清理   |
| **安全**            | "漏洞"、"安全扫描"、"以非 root 运行"              | 非 root 用户、密钥、最小化 |
| **Compose 搭建**    | "docker-compose"、"编排"、"多服务"                | 服务、网络、健康检查       |
| **网络**            | "连不上"、"找不到服务"、"DNS 解析"                | 网络配置、服务发现         |
| **审查**            | "审查我的 Docker"、"生产就绪？"、"最佳实践"       | 完整清单审计               |
| **开发**            | "热重载"、"开发容器"、"在 Docker 中调试"          | 开发目标、卷挂载           |

#### 需立即识别的常见反模式

- **没有 USER 指令的 root 用户** — 安全风险，优先修复
- **源码在依赖之前复制** — 破坏层缓存，构建缓慢
- **`npm install` 而非 `npm ci`** — 构建不可复现
- **ENV 或 COPY 中包含密钥** — 在镜像层中暴露
- **没有 .dockerignore** — 构建上下文臃肿，传输缓慢
- **基础镜像使用 `latest` 标签** — 构建不可复现
- **一个容器中运行多个服务** — 违反单一职责原则

### 阶段 3：方案设计 — 核心模式

**触发条件：** 问题分类完成后。根据用户技术栈应用相应模式。

#### 模式 1：多阶段构建（通用起点）

这是几乎所有生产 Dockerfile 的基础。将构建依赖与运行时产物分离：

```dockerfile
# 阶段 1：安装依赖（缓存友好）
FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

# 阶段 2：构建应用
FROM node:18-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build && npm prune --production

# 阶段 3：生产运行时（最小化）
FROM node:18-alpine AS runtime
RUN addgroup -g 1001 -S nodejs && adduser -S appuser -u 1001 -G nodejs
WORKDIR /app
COPY --from=deps --chown=appuser:nodejs /app/node_modules ./node_modules
COPY --from=build --chown=appuser:nodejs /app/dist ./dist
COPY --from=build --chown=appuser:nodejs /app/package*.json ./
USER appuser
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1
CMD ["node", "dist/index.js"]
```

**此模式的关键原则：**
- `deps` 阶段隔离依赖安装 → 仅在 package.json 变更时重新构建
- `build` 阶段包含完整工具链 → 与运行时分离
- `runtime` 阶段最小化 → 无构建工具、非 root 用户、包含健康检查
- COPY 上的 `--chown` 确保非 root 用户下的正确文件所有权

#### 模式 2：语言特定优化

针对每种语言生态调整多阶段模式：

**Go — 使用 scratch 或 distroless（无需运行时）：**
```dockerfile
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/server .

FROM gcr.io/distroless/static-debian12
COPY --from=build /app/server /server
USER nonroot:nonroot
EXPOSE 8080
CMD ["/server"]
```

**Python — 使用 virtualenv 和 slim 基础镜像：**
```dockerfile
FROM python:3.12-slim AS build
WORKDIR /app
COPY requirements.txt .
RUN pip install --user --no-cache-dir -r requirements.txt

FROM python:3.12-slim AS runtime
RUN useradd -m -u 1001 appuser
COPY --from=build --chown=appuser:appuser /root/.local /home/appuser/.local
COPY --chown=appuser:appuser . .
USER appuser
ENV PATH=/home/appuser/.local/bin:$PATH
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

**Java/Maven — 一个阶段构建，另一个阶段用 JRE 运行：**
```dockerfile
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -B
COPY src ./src
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine AS runtime
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=build --chown=appuser:appgroup /app/target/*.jar /app/app.jar
USER appuser
EXPOSE 8080
CMD ["java", "-jar", "/app/app.jar"]
```

#### 模式 3：Docker Compose 生产配置

```yaml
version: '3.8'
services:
  app:
    build:
      context: .
      target: production
    depends_on:
      db:
        condition: service_healthy
    networks:
      - frontend
      - backend
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB_FILE: /run/secrets/db_name
      POSTGRES_USER_FILE: /run/secrets/db_user
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_name
      - db_user
      - db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - backend
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$(cat /run/secrets/db_user)"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true

volumes:
  postgres_data:

secrets:
  db_name:
    external: true
  db_user:
    external: true
  db_password:
    external: true
```

**此 compose 文件的关键决策：**
- `condition: service_healthy` 确保数据库就绪后应用才启动
- `backend` 网络设为 `internal: true` — 数据库不对外部流量暴露
- 密钥使用 `_FILE` 变体 — 绝不直接放在环境变量中
- 资源限制防止任何服务独占资源
- `restart: unless-stopped` 保障生产弹性

#### 模式 4：开发环境覆盖

使用单独的 compose 覆盖文件用于开发：

```yaml
# docker-compose.override.yml（开发环境）
services:
  app:
    build:
      target: development
    volumes:
      - .:/app
      - /app/node_modules
    environment:
      - NODE_ENV=development
      - DEBUG=app:*
    ports:
      - "9229:9229"
    command: npm run dev
```

或在单个 compose 文件中使用基于 profile 的方式：

```yaml
services:
  app:
    build:
      context: .
      target: ${BUILD_TARGET:-production}
    volumes:
      - ${DEV_VOLUME:-}  # 仅在 .env.dev 中设置
    profiles:
      - ${PROFILE:-production}
```

#### 模式 5：安全加固清单

每个生产 Dockerfile 必须包含以下安全措施。按优先级顺序应用：

| 优先级 | 措施               | 实施方式                                         |
| ------ | ------------------ | ------------------------------------------------ |
| 🔴 P0   | 非 root 用户       | `USER 1001` 配合显式 UID/GID 创建                |
| 🔴 P0   | 镜像层中无密钥     | 仅使用 BuildKit 密钥挂载或运行时密钥             |
| 🟠 P1   | 最小化基础镜像     | Alpine 或 distroless，不使用完整 OS 镜像         |
| 🟠 P1   | 固定基础镜像摘要   | `FROM node:18-alpine@sha256:...` 而非标签        |
| 🟡 P2   | 定义 HEALTHCHECK   | HTTP 端点或进程检查                              |
| 🟡 P2   | 移除 Linux 权限    | 至少 `--cap-drop=ALL --cap-add=NET_BIND_SERVICE` |
| 🟢 P3   | 只读根文件系统     | `--read-only` 配合 tmpfs 用于可写路径            |
| 🟢 P3   | 运行时不含包管理器 | 仅将构建产物复制到生产阶段                       |

**BuildKit 密钥示例（不会留在镜像层中）：**
```dockerfile
# syntax=docker/dockerfile:1
FROM node:18-alpine
RUN --mount=type=secret,id=npm_token \
    NPM_TOKEN=$(cat /run/secrets/npm_token) \
    npm ci --only=production
```

#### 模式 6：使用 BuildKit 优化构建缓存

```dockerfile
# syntax=docker/dockerfile:1
FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --only=production
```

各包管理器的缓存挂载目标：

| 包管理器   | 缓存挂载目标                   |
| ---------- | ------------------------------ |
| npm        | `/root/.npm`                   |
| yarn       | `/usr/local/share/.cache/yarn` |
| pip        | `/root/.cache/pip`             |
| go modules | `/go/pkg/mod`                  |
| maven      | `/root/.m2`                    |
| apt        | `/var/cache/apt`               |

### 阶段 4：实施

**触发条件：** 方案设计确认后。编写或修改文件。

#### 步骤 4.1 - 确定文件创建策略

| 场景                   | 操作                              |
| ---------------------- | --------------------------------- |
| 不存在 Dockerfile      | 从零创建 `Dockerfile`             |
| 现有 Dockerfile 需优化 | 使用 `SearchReplace` 修改特定部分 |
| 多服务需要编排         | 创建或修改 `docker-compose.yml`   |
| 构建上下文缓慢         | 创建或更新 `.dockerignore`        |

#### 步骤 4.2 - .dockerignore 模板

始终确保存在全面的 `.dockerignore`。这通过减小上下文体积显著加速构建：

```
node_modules
.git
.gitignore
*.md
.git
.env
.env.*
dist
build
coverage
.nyc_output
*.log
.DS_Store
.vscode
.idea
docker-compose*.yml
Dockerfile*
```

根据项目技术栈调整——排除测试文件、文档和构建中会重新生成的任何目录。

#### 步骤 4.3 - 层顺序原则

编写或修改 Dockerfile 时，始终遵循以下顺序：

```
1. 基础镜像（FROM）           — 很少变更
2. 系统依赖（apt/apk）        — 很少变更
3. 包管理器文件（COPY）       — 偶尔变更
4. 依赖安装（RUN）            — 依赖变更时变更
5. 应用源码（COPY）           — 频繁变更
6. 构建步骤（RUN）            — 频繁变更
7. 运行时配置                 — 偶尔变更
```

**黄金法则：** 很少变更的内容放最前面。频繁变更的内容放最后面。

#### 常见层顺序错误

**错误（源码在依赖之前）：**
```dockerfile
COPY . .
RUN npm ci
```
每次代码变更都会使 npm 缓存失效 → 重新下载所有内容。

**正确（依赖在源码之前）：**
```dockerfile
COPY package*.json ./
RUN npm ci
COPY . .
```
仅在 `package.json` 变更时才重新运行 `npm ci`。

### 阶段 5：验证

**触发条件：** 每次 Dockerfile 或 compose 文件变更后。在认为任务完成前始终验证。

#### 步骤 5.1 - 构建验证

```bash
docker build --no-cache -t test-build .
```

如果构建失败：
- 检查基础镜像可用性（`docker pull <image>`）
- 验证 COPY 路径在构建上下文中存在
- 检查 RUN 命令的语法错误
- 确保多阶段 COPY --from 目标存在

#### 步骤 5.2 - 镜像体积检查

```bash
docker images test-build --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
docker history test-build --no-trunc --format "table {{.Size}}\t{{.CreatedBy}}" | head -10
```

| 体积状况 | 阈值      | 操作                              |
| -------- | --------- | --------------------------------- |
| 可接受   | < 500MB   | 对大多数应用来说良好              |
| 需关注   | 500MB-1GB | 检查镜像中是否有构建工具或缓存    |
| 过大     | > 1GB     | 应用多阶段构建，切换到 distroless |

#### 步骤 5.3 - 安全扫描（如可用）

```bash
docker scout quickview test-build 2>/dev/null || echo "Docker Scout not available"
```

如果 Docker Scout 不可用，建议用户使用其首选工具运行扫描。

#### 步骤 5.4 - 运行时验证

```bash
docker run --rm -d --name validate-test test-build
sleep 5
docker ps --filter name=validate-test --format "{{.Status}}"
docker logs validate-test --tail 20
docker stop validate-test
```

检查项：
- 容器启动后不会立即崩溃
- 应用绑定到预期端口
- 健康检查通过（如已定义）
- 无权限错误（USER 设置正确）
- 日志显示预期的启动信息

#### 步骤 5.5 - Compose 验证（如适用）

```bash
docker-compose config 2>/dev/null && echo "Compose config valid"
docker-compose up -d --build
docker-compose ps
docker-compose down
```

## 参考：代码审查清单

审查现有 Docker 配置时，逐项检查以下内容。按严重程度分类报告发现：

### Dockerfile 质量
- [ ] 依赖在源码之前复制以优化层缓存
- [ ] 多阶段构建分离了构建和运行时环境
- [ ] 生产阶段仅包含必要产物
- [ ] 通过全面的 .dockerignore 优化了构建上下文
- [ ] 基础镜像选择适合技术栈和约束
- [ ] RUN 命令在有益时合并以减少层数

### 容器安全
- [ ] 使用特定 UID/GID 创建了非 root 用户（非默认值）
- [ ] 容器以非 root 用户运行（存在 USER 指令）
- [ ] 密钥管理得当（不在环境变量或镜像层中）
- [ ] 基础镜像保持更新并扫描漏洞
- [ ] 最小攻击面（仅安装必要包）
- [ ] 实现了健康检查用于容器监控

### Docker Compose 与编排
- [ ] 服务依赖通过健康检查正确定义
- [ ] 配置了自定义网络用于服务隔离
- [ ] 环境特定配置已分离（开发/生产）
- [ ] 卷策略适合数据持久化需求
- [ ] 定义了资源限制以防止资源耗尽
- [ ] 配置了重启策略以保障生产弹性

### 性能与体积
- [ ] 最终镜像体积在 500MB 以下（除非有正当理由）
- [ ] 实现了构建缓存优化（挂载或层顺序）
- [ ] 如需要已考虑多架构构建
- [ ] 选择性复制产物（仅必要文件）
- [ ] 包管理器缓存在同一 RUN 层中清理

### 开发工作流
- [ ] 开发目标与生产分离
- [ ] 通过卷挂载正确配置了热重载
- [ ] 需要时暴露了调试端口
- [ ] 不同阶段的环境变量配置正确
- [ ] 测试容器与生产构建隔离

## 参考：常见问题排查

### 构建性能问题
**症状**：构建缓慢（10+ 分钟）、频繁缓存失效
**根因**：层顺序不当、构建上下文过大、无缓存策略
**修复**：重新排序层（依赖在源码之前）、添加 .dockerignore、使用 BuildKit 缓存挂载

### 安全漏洞
**症状**：安全扫描失败、密钥暴露、以 root 执行
**根因**：基础镜像过时、硬编码密钥、默认用户
**修复**：固定基础镜像摘要、使用 BuildKit 密钥、添加非 root USER 指令

### 镜像体积问题
**症状**：镜像超过 1GB、部署缓慢
**根因**：不必要文件、生产中包含构建工具、基础镜像选择不当
**修复**：切换到 distroless、多阶段优化、选择性产物复制

### 网络问题
**症状**：服务通信失败、DNS 解析错误
**根因**：缺少网络定义、端口冲突、服务命名
**修复**：定义自定义网络、添加健康检查配合 `depends_on: condition: service_healthy`

### 开发工作流问题
**症状**：热重载失败、调试困难、迭代缓慢
**根因**：卷挂载问题、端口配置、环境不匹配
**修复**：创建开发专用目标、合理的卷策略、调试配置

## 参考：集成边界

**何时推荐其他技能/专家：**
- **Kubernetes Pod、Service、Ingress** → 不在范围内。Docker 运行单个容器；K8s 编排集群。
- **使用容器的 CI/CD 流水线** → 将此技能与 GitHub Actions 或 CI 平台专业知识结合。
- **云特定容器服务（ECS、Fargate、Cloud Run）** → Docker 模式适用，但部署细节需要云专业知识。
- **复杂持久化的数据库容器化** → 此处提供基础模式；复杂备份/HA 策略需要数据库专业知识。
