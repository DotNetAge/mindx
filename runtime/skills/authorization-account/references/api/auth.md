# 第一部分:授权服务(auth-service,端口 8181)

授权服务在本技能中仅用于登录获取访问令牌。用户注册、退出、改密、用户列表等管理接口不属于本技能范围。

## 通用响应格式

业务接口 HTTP 状态码恒为 200,以 `code` 字段判断结果(`0` 为成功);JWT 中间件校验失败时返回真实 HTTP 401 与 `{"error":"..."}`。

- 成功单条:`{"code":0,"msg":"success","data":<对象>}`
- 失败:`{"code":<400|404|500>,"msg":"<错误信息>"}`

## POST /auth/login — 用户登录(获取令牌)

无需认证。请求体(两个字段均**必填**,后端无 required 校验,缺失将返回 `code:400` 绑定错误):

```json
{ "name": "alice", "password": "secret123" }
```

成功响应(`data` 为**令牌对象**,其中的 `access_token` 才是 JWT 字符串,后续请求放入 `Authorization: Bearer <data.access_token>`):

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "scope": "user",
    "expires_in": 7200000000000
  }
}
```

错误:用户不存在或密码错误时返回 `code:500`。

## JWT 令牌说明

- 登录响应 `data.access_token` 为 JWT 字符串,请求时放入 `Authorization: Bearer <access_token>`。
- 令牌载荷(claims)中包含 `user_id` 字段,即当前用户 ID,与账号服务的账户 ID 一致。
- 令牌有效期 2 小时(`access_token` 2h / `refresh_token` 4h),过期后需重新登录获取新令牌。

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /health | 健康检查 |
| GET | /ready | 就绪检查 |
