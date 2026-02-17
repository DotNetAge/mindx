---
name: n8n
description: n8n工作流管理技能，管理工作流自动化、执行记录和凭证
version: 1.0.0
category: general
tags:
  - n8n
  - workflow
  - automation
  - api
  - 工作流
  - 自动化
  - n8n
os:
  - darwin
  - linux
enabled: true
timeout: 60
command: curl
requires:
  bins:
    - curl
  env:
    - N8N_API_KEY
    - N8N_HOST
homepage: https://docs.n8n.io/api/
---

# n8n 工作流自动化技能

通过 n8n 公共 REST API 管理工作流、执行记录、标签和凭证。

## 快速开始

```bash
# 设置环境变量
export N8N_API_KEY="your-api-key-here"
export N8N_HOST="http://localhost:5678"  # 或你的n8n Cloud实例

# 获取所有活跃工作流
curl "$N8N_HOST/api/v1/workflows?active=true" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "accept: application/json"
```

## 认证方式

### 获取 API 密钥

1. 登录 n8n 实例
2. 进入 **Settings** → **n8n API**
3. 点击 **Create an API Key**
4. 设置标签和过期时间
5. 复制生成的 API 密钥

### 在请求中使用

```bash
curl "$N8N_HOST/api/v1/workflows" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "accept: application/json"
```

## 核心 API 端点

### 工作流管理

- 获取工作流列表: `GET /api/v1/workflows`
- 获取单个工作流: `GET /api/v1/workflows/{id}`
- 创建工作流: `POST /api/v1/workflows`
- 更新工作流: `PATCH /api/v1/workflows/{id}`
- 删除工作流: `DELETE /api/v1/workflows/{id}`

### 执行管理

- 获取执行记录: `GET /api/v1/executions`
- 获取单个执行: `GET /api/v1/executions/{id}`
- 删除执行记录: `DELETE /api/v1/executions/{id}`

### 手动触发工作流

```bash
curl -X POST "$N8N_HOST/api/v1/workflows/{workflow-id}/execute" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"data": {"key1": "value1"}}'
```

## 注意事项

- n8n API 在免费试用期间不可用，需要升级后才能访问
- 当前使用 v1 API，路径为 `/api/v1/`
- 必须在请求头中包含 `X-N8N-API-KEY`
- 详细 API 文档见 [API_REFERENCE.md](references/API_REFERENCE.md)
