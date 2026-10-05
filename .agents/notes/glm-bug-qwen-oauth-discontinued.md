# Qwen OAuth 官方下线与注册表清理

日期：2026-10-05
背景：MindX Work 设置页"接入 Qwen Code"报 `invalid character '<' looking for beginning of value`。

## 根因（证据链完整）

- Qwen OAuth 免费层已于 **2026-04-15 被官方下线**（qwen-code-docs Troubleshooting 明文："Qwen OAuth is no longer available as of April 15, 2026"）
- 实测（curl 与 Node 双栈）：`https://chat.qwen.ai/api/v1/oauth2/device/code` 仍正常返回 JSON（遗留端点），但 `/api/v1/oauth2/token` 对**任何 POST**（任意 grant_type/UA/头组合）一律返回 HTTP 405 + 阿里云 WAF HTML 拦截页
- 故 device_code 流能发码、永远换不到 token；gochat 对 HTML 做 json.Unmarshal 报 "invalid character '<'"

## 排查中排除的假设（勿重试）

- 不是 PKCE/UA/编码方式/Accept/Origin 头的问题——全部 405
- 不是 Go 客户端 TLS 指纹——Node fetch 同样被拦
- 不是 grant_type 值触发——authorization_code/refresh_token 同样 405（端点级阻断）

## 处置（用户选 B：直接删除）

- runtime/settings/client-provider.yml 删除 qwen 条目，留两行注释说明下线原因与日期，防止未来照抄旧版加回
- oauth_providers_test.go 同步：qwen 断言改为"不存在"，kind/interaction 派生断言移到 minimax；测试全过
- setup.go 的 SyncEmbeddedFile 对该 yml 是强制覆盖同步，重新构建 + make restart 后用户机器即生效
- gochat 的 QwenProvider（auth/qwen.go）是公开 API 未动；官方 CLI 的替代路径是 API Key / Coding Plan

## 经验

1. "invalid character '<'" 类错误先 curl 复现再看代码——本例 10 分钟定位到服务端，若先读 gochat 源码会浪费在死路上
2. OAuth provider 注册表里的条目会死（服务方下线），技能引用 metadata.requires.oauth.provider 的技能同样会死；处置时测试断言与 yml 必须一起改
3. 判断报错来源的顺序：HTTP 状态码 + Content-Type → 客户端指纹（换栈复现）→ 服务端策略（搜官方文档）；本例最后一步才见分晓
