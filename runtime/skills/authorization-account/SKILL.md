---
name: authorization-account
description: 当用户需要登录 Factor 平台获取访问令牌、读取当前用户资料或更新当前用户资料时应使用此技能。本技能提供独立脚本 scripts/cli.py,login 命令输出完整 JSON(含 data.access_token),令牌不落盘而是暂存于对话上下文,供本技能及其它技能以 --token 参数复用。不适用于用户管理、角色管理、客户端管理、资源管理等管理接口。
metadata:
  name_zh: 授权与账号
---

# 授权与账号(Factor)

## 目的

本技能指导 Agent 完成 Factor 平台的登录鉴权与个人账号数据读写:

- **登录**:获取 JWT 访问令牌(授权服务,端口 8181)。
- **个人账号**:读取/更新当前用户资料、更新头像(账号服务,端口 8182)。

**调用约定:一切操作通过 python 指令执行,禁止手工裸调 HTTP API。**

**令牌约定(重要)**:`login` 不保存任何文件,而是把登录响应的**完整 JSON** 作为结果输出;Agent 从输出中取出 `data.access_token`,暂存于**对话上下文**。之后本技能及其它技能的所有命令都通过 `--token <访问令牌>` 参数传入,实现跨技能复用。

**范围边界**:只包含"登录获取令牌"与"读取/更新当前用户资料"两类操作;`users`、`roles`、`clients`、`resources` 等管理接口均不属于本技能范围。

## 技能里有什么

| 位置(相对技能目录)    | 内容                                                   |
| --------------------- | ------------------------------------------------------ |
| `SKILL.md`            | 本文件:技能总览、怎么用                                |
| `scripts/cli.py`      | python 命令行入口:login / me / account-update / avatar |
| `references/usage.md` | 使用说明:四个命令的参数、规则、示例                    |
| `references/api/`     | 裸 HTTP API 存档(仅供底层信息查询,不用于调用)          |

## 怎么用

在**本技能目录**下执行:

```
# 1. 登录:输出完整 JSON(不保存文件)
python ./scripts/cli.py login 用户名 密码

# 2. 从输出中取出 data.access_token,暂存于对话上下文,后续命令以 --token 传入
python ./scripts/cli.py --token <访问令牌> me

# 3. 更新资料(只传要改的字段)
python ./scripts/cli.py --token <访问令牌> account-update real_name=爱丽丝 mobile=13800000000

# 4. 更新头像
python ./scripts/cli.py --token <访问令牌> avatar https://example.com/avatar.png
```

**每个命令的参数、必填字段与示例:读 `references/usage.md`。**

## 令牌跨技能复用

其他技能(如「数据管理」)不重复登录,直接复用本技能获取的令牌:

```
# 先在「授权与账号」技能目录登录,取 data.access_token
python ./scripts/cli.py login 用户名 密码

# 再到目标技能目录,以 --token 传入
python ./scripts/api_client/cli.py --token <访问令牌> customer-list --size 20
```

## 调用流程

1. `login 用户名 密码` 获取令牌,把返回的完整 JSON 输出,取 `data.access_token` 暂存上下文。
2. `me --token <令牌>` 读取当前用户资料。
3. 需要修改时 `account-update --token <令牌>` / `avatar --token <令牌>`。

## 关键约定

- **账户 ID 与用户 ID 一致**,由注册事件自动创建,无需也无法通过本技能创建。
- **`user_name` 不可修改**,`avatar_url` 不生效(头像必须走 `avatar` 命令)。
- **令牌有效期 2 小时**,过期后重新 `login`;错误提示会引导重新登录。
- **令牌不落盘**,只暂存于对话上下文;对话结束令牌即失效,新会话需重新登录。
- **密码为明文**,注意数据敏感性。
