# 授权与账号:使用说明

本技能的一切操作通过本技能的 python 命令入口执行(命令在**本技能目录**下执行):

```
python ./scripts/cli.py <命令> [参数]
```

禁止手工裸调 HTTP API;裸 API 说明仅作存档(`api/`),仅供底层信息查询。

## 令牌模型(必读)

- `login` **不保存任何文件**,把登录响应的完整 JSON 原样输出。
- Agent 从输出中取 `data.access_token`,**暂存于对话上下文**。
- 本技能其它命令及**其它技能**的所有命令都以 `--token <访问令牌>` 传入。
- 令牌有效期 2 小时,过期(HTTP 401)后重新 `login`。

## 命令说明

### login 用户名 密码

登录获取令牌,输出完整 JSON(不落盘)。

```
python ./scripts/cli.py login 用户名 密码
```

- 两参数均必填;失败提示"业务错误",请检查用户名/密码。
- 输出示例(取 `data.access_token` 使用):

```json
{
  "code": 0,
  "data": {
    "access_token": "eyJhbGciOi...",
    "refresh_token": "...",
    "scope": "api",
    "expires_in": 7200
  }
}
```

### me --token <访问令牌>

读取当前用户资料(账户 ID 与用户 ID 一致,自动从 JWT 解码)。

返回字段:`id`、`user_name`、`real_name`、`avatar_url`、`email`、`mobile`、`warehouse_id`、`warehouse_name`、`bio`、`created_at`、`updated_at`。

```
python ./scripts/cli.py --token <访问令牌> me
```

### account-update --token <访问令牌> 字段=值 ...

更新当前用户资料,**只传要修改的字段**;内部"先查后写"合并现有资料,不会清空未提供的字段。

```
python ./scripts/cli.py --token <访问令牌> account-update real_name=爱丽丝 mobile=13800000000
python ./scripts/cli.py --token <访问令牌> account-update bio=新的个人介绍
```

| 参数 | 规则 |
|---|---|
| `real_name=...` | 必填(非空,≤50 字符) |
| `mobile=...` | 必填,11 位手机号格式 |
| `email=...` | 可选,传了需符合邮箱格式 |
| `warehouse_id=...` / `warehouse_name=...` | 可选 |
| `bio=...` | 可选 |

注意:`user_name` 不可修改;`avatar_url` 不生效(会报引导错误),头像请用 `avatar` 命令。

### avatar --token <访问令牌> 头像URL

更新当前用户头像,自动处理请求体的驼峰 `avatarUrl` 字段。

```
python ./scripts/cli.py --token <访问令牌> avatar https://example.com/avatar.png
```

## 关键约定

- **账户 ID 与用户 ID 一致**,由注册事件自动创建,无需也无法创建。
- **`user_name` 不可修改**,头像必须走 `avatar` 命令。
- **令牌有效期 2 小时**,过期后重新 `login`;错误提示会引导重新登录。
- **令牌不落盘**,只暂存于对话上下文;对话结束令牌即失效,新会话需重新登录。
- **密码为明文**,注意数据敏感性。
- 所有失败输出「问题→原因→下一步」引导提示,按提示修正重试。
