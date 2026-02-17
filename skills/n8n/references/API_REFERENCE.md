# n8n API 完整参考文档

n8n公共REST API的完整端点参考。

## API 概述

- **基础URL**: `<N8N_HOST>/api/v1`
- **认证方式**: API密钥（Header: `X-N8N-API-KEY`）
- **内容类型**: `application/json`
- **版本**: 当前使用v1

## 认证

### API密钥

所有API请求必须在请求头中包含API密钥：

```bash
curl "$N8N_HOST/api/v1/workflows" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "accept: application/json"
```

### 获取API密钥

1. 登录n8n实例
2. 进入 **Settings** → **n8n API**
3. 点击 **Create an API Key**
4. 设置标签和过期时间
5. 复制生成的API密钥

### 环境变量

推荐使用环境变量存储敏感信息：

```bash
export N8N_API_KEY="your-api-key-here"
export N8N_HOST="http://localhost:5678"
# 或 n8n Cloud
# export N8N_HOST="https://your-instance.app.n8n.cloud"
```

## Workflows API

### GET /workflows

获取工作流列表。

**端点:** `GET /api/v1/workflows`

**查询参数:**

| 参数 | 类型 | 必需 | 描述 | 默认值 |
|------|------|------|------|--------|
| `active` | boolean | 否 | 仅返回活跃工作流 | false |
| `page` | integer | 否 | 页码 | 1 |
| `pageSize` | integer | 否 | 每页条数 | 20 |

**示例:**
```bash
curl "$N8N_HOST/api/v1/workflows?active=true&page=1&pageSize=50" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

**响应:**
```json
{
  "data": [
    {
      "id": "workflow-id",
      "name": "My Workflow",
      "active": true,
      "nodes": [],
      "connections": {},
      "settings": {},
      "staticData": null,
      "tags": [],
      "versionId": "version-id"
    }
  ],
  "nextCursor": "cursor-token"
}
```

### GET /workflows/{id}

获取单个工作流。

**端点:** `GET /api/v1/workflows/{id}`

**路径参数:**
- `id`: 工作流ID

**示例:**
```bash
curl "$N8N_HOST/api/v1/workflows/{workflow-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### GET /workflows/name/{name}

通过名称获取工作流。

**端点:** `GET /api/v1/workflows/name/{name}`

**路径参数:**
- `name`: 工作流名称

**示例:**
```bash
curl "$N8N_HOST/api/v1/workflows/name/My%20Workflow" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### POST /workflows

创建新工作流。

**端点:** `POST /api/v1/workflows`

**请求体:**
```json
{
  "name": "My Workflow",
  "nodes": [
    {
      "name": "Start",
      "type": "n8n-nodes-base.start",
      "position": [240, 300],
      "parameters": {}
    }
  ],
  "connections": {},
  "settings": {},
  "staticData": null,
  "tags": [],
  "pinData": null
}
```

**示例:**
```bash
curl -X POST "$N8N_HOST/api/v1/workflows" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Workflow",
    "nodes": [
      {
        "name": "Start",
        "type": "n8n-nodes-base.start",
        "position": [240, 300],
        "parameters": {}
      }
    ],
    "connections": {},
    "settings": {}
  }'
```

### PATCH /workflows/{id}

更新工作流。

**端点:** `PATCH /api/v1/workflows/{id}`

**路径参数:**
- `id`: 工作流ID

**请求体:**
```json
{
  "name": "Updated Name",
  "active": true,
  "settings": {}
}
```

**示例:**
```bash
curl -X PATCH "$N8N_HOST/api/v1/workflows/{workflow-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "active": true
  }'
```

### DELETE /workflows/{id}

删除工作流。

**端点:** `DELETE /api/v1/workflows/{id}`

**示例:**
```bash
curl -X DELETE "$N8N_HOST/api/v1/workflows/{workflow-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

## Executions API

### GET /executions

获取执行记录列表。

**端点:** `GET /api/v1/executions`

**查询参数:**

| 参数 | 类型 | 必需 | 描述 | 默认值 |
|------|------|------|------|--------|
| `workflowId` | string | 否 | 工作流ID | - |
| `status` | string | 否 | 执行状态 (success/error/running) | - |
| `page` | integer | 否 | 页码 | 1 |
| `pageSize` | integer | 否 | 每页条数 | 20 |

**示例:**
```bash
# 获取所有执行记录
curl "$N8N_HOST/api/v1/executions" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"

# 获取特定工作流的执行记录
curl "$N8N_HOST/api/v1/executions?workflowId={workflow-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"

# 按状态筛选
curl "$N8N_HOST/api/v1/executions?status=success" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### GET /executions/{id}

获取单个执行记录。

**端点:** `GET /api/v1/executions/{id}`

**示例:**
```bash
curl "$N8N_HOST/api/v1/executions/{execution-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### DELETE /executions/{id}

删除执行记录。

**端点:** `DELETE /api/v1/executions/{id}`

**示例:**
```bash
curl -X DELETE "$N8N_HOST/api/v1/executions/{execution-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### DELETE /executions

删除所有执行记录。

**端点:** `DELETE /api/v1/executions`

**查询参数:**

| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `before` | string | 否 | 删除此时间之前的记录 (ISO格式) |

**示例:**
```bash
# 删除所有执行记录
curl -X DELETE "$N8N_HOST/api/v1/executions" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"

# 删除30天前的记录
CUTOFF_DATE=$(date -d "30 days ago" -u +"%Y-%m-%dT%H:%M:%S.%3NZ")
curl -X DELETE "$N8N_HOST/api/v1/executions?before=$CUTOFF_DATE" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

## Workflow Execution API

### POST /workflows/{id}/execute

通过ID执行工作流。

**端点:** `POST /api/v1/workflows/{id}/execute`

**路径参数:**
- `id`: 工作流ID

**请求体:**
```json
{
  "data": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

**示例:**
```bash
curl -X POST "$N8N_HOST/api/v1/workflows/{workflow-id}/execute" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "input": "test data"
    }
  }'
```

### POST /workflows/name/{name}/execute

通过名称执行工作流。

**端点:** `POST /api/v1/workflows/name/{name}/execute`

**路径参数:**
- `name`: 工作流名称

**请求体:**
```json
{
  "data": {
    "key1": "value1"
  }
}
```

**示例:**
```bash
curl -X POST "$N8N_HOST/api/v1/workflows/name/My%20Workflow/execute" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "message": "Hello"
    }
  }'
```

## Tags API

### GET /tags

获取所有标签。

**端点:** `GET /api/v1/tags`

**示例:**
```bash
curl "$N8N_HOST/api/v1/tags" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### POST /tags

创建标签。

**端点:** `POST /api/v1/tags`

**请求体:**
```json
{
  "name": "My Tag",
  "createdAt": "2024-01-01T00:00:00.000Z"
}
```

**示例:**
```bash
curl -X POST "$N8N_HOST/api/v1/tags" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production",
    "createdAt": "2024-01-01T00:00:00.000Z"
  }'
```

### DELETE /tags/{id}

删除标签。

**端点:** `DELETE /api/v1/tags/{id}`

**示例:**
```bash
curl -X DELETE "$N8N_HOST/api/v1/tags/{tag-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

## Credentials API

### GET /credentials

获取所有凭证。

**端点:** `GET /api/v1/credentials`

**示例:**
```bash
curl "$N8N_HOST/api/v1/credentials" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### GET /credentials/{id}

获取单个凭证。

**端点:** `GET /api/v1/credentials/{id}`

**示例:**
```bash
curl "$N8N_HOST/api/v1/credentials/{credential-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

### POST /credentials

创建凭证。

**端点:** `POST /api/v1/credentials`

**请求体:**
```json
{
  "name": "My Credential",
  "type": "slackApi",
  "data": {
    "accessToken": "xoxb-your-token"
  }
}
```

**示例:**
```bash
curl -X POST "$N8N_HOST/api/v1/credentials" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Slack API",
    "type": "slackApi",
    "data": {
      "accessToken": "xoxb-your-token"
    }
  }'
```

### PATCH /credentials/{id}

更新凭证。

**端点:** `PATCH /api/v1/credentials/{id}`

**请求体:**
```json
{
  "name": "Updated Name"
}
```

**示例:**
```bash
curl -X PATCH "$N8N_HOST/api/v1/credentials/{credential-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Credential Name"
  }'
```

### DELETE /credentials/{id}

删除凭证。

**端点:** `DELETE /api/v1/credentials/{id}`

**示例:**
```bash
curl -X DELETE "$N8N_HOST/api/v1/credentials/{credential-id}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

## 错误处理

### HTTP状态码

| 状态码 | 含义 | 常见原因 |
|--------|------|---------|
| 200 | 成功 | 请求成功 |
| 400 | 错误请求 | 参数错误、格式错误 |
| 401 | 未授权 | API密钥无效或缺失 |
| 403 | 禁止访问 | 权限不足 |
| 404 | 未找到 | 资源不存在 |
| 500 | 服务器错误 | n8n内部错误 |

### 错误响应格式

```json
{
  "message": "Error message",
  "statusCode": 400
}
```

## 分页

大多数列表端点支持分页参数：

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `page` | 页码 | 1 |
| `pageSize` | 每页条数 | 20 |

**分页响应:**
```json
{
  "data": [...],
  "nextCursor": "cursor-token"
}
```

**使用cursor获取下一页:**
```bash
curl "$N8N_HOST/api/v1/workflows?cursor={next-cursor}" \
  -H "X-N8N-API-KEY: $N8N_API_KEY"
```

## 最佳实践

### 1. 环境变量管理

```bash
# ~/.bashrc 或 ~/.zshrc
export N8N_API_KEY="your-api-key"
export N8N_HOST="http://localhost:5678"

# 重新加载配置
source ~/.bashrc
```

### 2. 错误处理

```bash
# 检查响应状态
RESPONSE=$(curl -s -w "\n%{http_code}" "$N8N_HOST/api/v1/workflows" \
  -H "X-N8N-API-KEY: $N8N_API_KEY")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [ "$HTTP_CODE" -eq 200 ]; then
  echo "请求成功: $BODY"
else
  echo "请求失败 (HTTP $HTTP_CODE): $BODY"
fi
```

### 3. 批量操作

```bash
# 批量激活工作流
curl -s "$N8N_HOST/api/v1/workflows?active=false" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" | \
  jq -r '.data[].id' | \
  while read id; do
    echo "激活: $id"
    curl -s -X PATCH "$N8N_HOST/api/v1/workflows/$id" \
      -H "X-N8N-API-KEY: $N8N_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{"active": true}'
  done
```

### 4. 执行工作流并等待

```bash
# 触发工作流
EXECUTION=$(curl -s -X POST "$N8N_HOST/api/v1/workflows/{id}/execute" \
  -H "X-N8N-API-KEY: $N8N_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"data": {"input": "test"}}')

EXECUTION_ID=$(echo "$EXECUTION" | jq -r '.data.executionId')

# 轮询执行状态
while true; do
  STATUS=$(curl -s "$N8N_HOST/api/v1/executions/$EXECUTION_ID" \
    -H "X-N8N-API-KEY: $N8N_API_KEY" | \
    jq -r '.data.finished')

  if [ "$STATUS" = "true" ]; then
    echo "执行完成"
    curl -s "$N8N_HOST/api/v1/executions/$EXECUTION_ID" \
      -H "X-N8N-API-KEY: $N8N_API_KEY" | jq '.'
    break
  fi

  echo "等待执行完成..."
  sleep 2
done
```

## 注意事项

1. **API可用性**: n8n API在免费试用期间不可用，需要升级后才能访问
2. **API版本**: 当前使用v1 API，路径为 `/api/v1/`
3. **认证**: 所有请求必须在Header中包含 `X-N8N-API-KEY`
4. **环境变量**: 推荐使用环境变量存储 `N8N_API_KEY` 和 `N8N_HOST`
5. **请求限制**: 遵守API速率限制，避免过多请求
6. **数据安全**: 不要在日志或版本控制中暴露API密钥

## 相关资源

- n8n官方文档: https://docs.n8n.io/api/
- API参考: https://docs.n8n.io/api/api-reference/
- n8n GitHub: https://github.com/n8n-io/n8n
