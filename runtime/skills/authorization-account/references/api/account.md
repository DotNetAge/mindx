# 第二部分:账号服务(account-service,端口 8182)

账号服务在本技能中仅用于读取当前用户资料与更新当前用户资料。**全部 `/accounts` 接口均需 JWT 认证**(`Authorization: Bearer <token>`)。

账户在用户注册时由 NATS 事件流以**与用户 ID 相同的 ID** 自动创建,本技能不提供账户创建接口。

## 当前用户 ID

当前用户 ID 即 JWT 载荷(claims)中的 `user_id` 字段,解码令牌即可获得,作为 `/accounts/:id` 中的 `:id` 使用。

## 端点清单

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /accounts/:id | 读取当前用户资料(:id 为当前用户 ID) |
| PUT | /accounts/:id | 更新当前用户资料 |
| PUT | /accounts/:id/avatar | 更新当前用户头像 |

## GET /accounts/:id — 读取当前用户资料

示例:`GET /accounts/<user_id>`。成功响应:

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "id": "0a1b2c3d-...",
    "user_name": "alice",
    "real_name": "爱丽丝",
    "avatar_url": "https://example.com/avatar.png",
    "email": "alice@example.com",
    "mobile": "13800000000",
    "warehouse_id": "wh1",
    "warehouse_name": "华东一号仓",
    "bio": "个人介绍",
    "created_at": "2026-08-06T09:00:00.123456789+08:00",
    "updated_at": "2026-08-06T09:00:00.123456789+08:00"
  }
}
```

资源不存在时返回 `code:404`。

## PUT /accounts/:id — 更新当前用户资料

**注意**:仅以下字段会生效:`real_name`、`email`、`mobile`、`warehouse_id`、`warehouse_name`、`bio`;`user_name` 不可修改(创建时取自注册事件),`avatar_url` 在 PUT 中不生效(头像必须走 `/avatar` 端点)。请求体(字段均为下划线风格):

```json
{
  "real_name": "爱丽丝",
  "email": "alice@example.com",
  "mobile": "13800000000",
  "warehouse_id": "wh1",
  "warehouse_name": "华东一号仓",
  "bio": "个人介绍"
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| real_name | string | 是 | 真实姓名(≤50 字符) |
| mobile | string | 是 | 手机号(需符合手机号格式) |
| email | string | 否 | 邮箱(传了则需符合邮箱格式) |
| warehouse_id | string | 否 | 仓库Id |
| warehouse_name | string | 否 | 仓库名称(前端界面只读展示,一般由前端带出) |
| bio | string | 否 | 个人介绍 |

> 必填依据前端表单规则;后端无 required 校验,缺失不报错但会产生不完整资料。

## PUT /accounts/:id/avatar — 更新当前用户头像

请求体(注意字段为驼峰 `avatarUrl`):

```json
{ "avatarUrl": "https://example.com/new-avatar.png" }
```

## 常见错误排查

| 现象 | 可能原因 |
|---|---|
| 响应为 `{"error":"令牌已过期或尚未生效 (Expired/Not Valid Yet)"}` | JWT 过期(2 小时),重新调用登录获取新令牌 |
| 返回 `code:404` | `:id` 与当前用户 ID 不一致,或账户尚未由注册事件创建 |

> 说明:响应中的 `created_at`/`updated_at` 为服务端 `time.Time` 输出(RFC3339Nano,通常带小数秒与 `+08:00` 偏移),仅供读取。
