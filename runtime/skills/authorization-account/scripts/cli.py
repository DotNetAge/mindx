"""授权与账号命令行入口(供 Agent 使用):登录获取令牌、读写当前用户资料。

用法(在技能目录下执行):
    python ./scripts/cli.py login <用户名> <密码>
    python ./scripts/cli.py me --token <访问令牌>
    python ./scripts/cli.py account-update --token <访问令牌> real_name=爱丽丝
    python ./scripts/cli.py avatar --token <访问令牌> https://example.com/avatar.png

设计约定(重要):
- login 不保存任何文件,直接把登录响应的完整 JSON 作为结果输出;
  Agent 从输出中取出 data.access_token,并暂存于对话上下文。
- 其它命令通过 --token <访问令牌> 传入令牌,实现跨技能复用。
- 令牌有效期 2 小时,过期(HTTP 401)后重新 login 获取新令牌。
"""
from __future__ import annotations

import argparse
import base64
import json
import re
import sys
from typing import Any, Dict, Optional

import requests

DEFAULT_TIMEOUT = 30
DEFAULT_AUTH_BASE = "http://localhost:8181"      # 授权服务(auth-service)
DEFAULT_ACCOUNT_BASE = "http://localhost:8182"   # 账号服务(account-service)

# 账户资料字段与中文标签(先查后写合并时使用)
ACCOUNT_FIELDS = {
    "user_name": "用户名",
    "real_name": "真实姓名",
    "avatar_url": "头像地址",
    "email": "邮箱",
    "mobile": "手机号",
    "warehouse_id": "仓库Id",
    "warehouse_name": "仓库名称",
    "bio": "个人介绍",
}


class FactorError(Exception):
    """引导性错误:「问题 -> 原因 -> 下一步」,便于 Agent 直接理解并修正。"""

    def __init__(self, func: str, message: str, reason: str, hint: str):
        self.func = func
        self.message = message
        self.reason = reason
        self.hint = hint
        super().__init__(self.__str__())

    def __str__(self) -> str:
        return (f"调用 {self.func} 时出现问题:{self.message}。"
                f"原因:{self.reason}。下一步:{self.hint}")


def decode_jwt_payload(token: str) -> Dict[str, Any]:
    """解码 JWT 的 payload 部分(不校验签名),用于读取 user_id 等声明。"""
    try:
        parts = token.split(".")
        if len(parts) != 3:
            raise ValueError
        padding = "=" * (-len(parts[1]) % 4)
        raw = base64.urlsafe_b64decode(parts[1] + padding)
        return json.loads(raw)
    except Exception:
        raise FactorError(
            func="解析令牌", message="无法解码访问令牌",
            reason="传入的令牌不是合法的 JWT 字符串,或已被截断",
            hint="请使用 login 输出中的 data.access_token 原文,不要手工拼写令牌",
        )


def _user_id(token: str) -> str:
    """从令牌载荷取 user_id(账户 ID 与用户 ID 一致)。"""
    payload = decode_jwt_payload(token)
    uid = payload.get("user_id") or payload.get("uid")
    if not uid:
        raise FactorError(
            func="解析令牌", message="令牌中缺少 user_id 声明",
            reason="该令牌不是有效的用户访问令牌",
            hint="请重新通过 login 获取令牌",
        )
    return str(uid)


def _print(obj) -> None:
    """完整输出 JSON(不截断),保证登录令牌等信息原样呈现。"""
    print(json.dumps(obj, ensure_ascii=False, indent=2, default=str))


def _convert(s: str):
    """把命令行字符串转为最合理的类型(int/float/bool/str)。"""
    if s in ("true", "false"):
        return s == "true"
    try:
        return int(s)
    except ValueError:
        pass
    try:
        return float(s)
    except ValueError:
        pass
    return s


def _parse_kv(pairs) -> Dict[str, Any]:
    """解析 字段=值 参数列表为字典。"""
    data: Dict[str, Any] = {}
    for p in pairs:
        if "=" not in p:
            raise FactorError(func="参数解析", message=f"参数格式应为 字段=值,收到:{p}",
                              reason="命令行参数缺少等号", hint="请使用 字段=值 形式传参")
        k, v = p.split("=", 1)
        data[k] = _convert(v)
    return data


def _check_required(func: str, data: Dict[str, Any], required: Dict[str, str]) -> None:
    """必填检查:缺失时抛出引导性错误。"""
    missing = [f"{key}({label})" for key, label in required.items() if not data.get(key)]
    if missing:
        keys = "、".join(missing)
        raise FactorError(func=func, message=f"缺少必填字段:{keys}",
                          reason=f"调用 {func} 时必填参数未提供或为空",
                          hint="请补充必填字段后重试")


def _check_email(func: str, value: Optional[str]) -> None:
    """邮箱格式检查(传了才检查)。"""
    if value is None or value == "":
        return
    if not re.match(r"^[^@\s]+@[^@\s]+\.[^@\s]+$", str(value)):
        raise FactorError(func=func, message=f"邮箱格式不合法:{value}",
                          reason="传入的邮箱缺少 @ 或域名部分",
                          hint="请改成合法邮箱格式,如 a@example.com")


def _check_phone(func: str, value: Optional[str]) -> None:
    """手机号格式检查(传了才检查,中国大陆 11 位手机号)。"""
    if value is None or value == "":
        return
    if not re.match(r"^1[3-9]\d{9}$", str(value)):
        raise FactorError(func=func, message=f"手机号格式不合法:{value}",
                          reason="手机号应为 11 位数字,以 1 开头",
                          hint="请改成合法的 11 位手机号")


def _parse(func: str, resp: requests.Response) -> Dict[str, Any]:
    """解析响应:业务接口 HTTP 恒 200 按 code 判断;JWT 失败是真实 401。"""
    if resp.status_code == 401:
        err = "令牌已过期或尚未生效"
        try:
            body = resp.json()
            err = body.get("error") or err
        except ValueError:
            pass
        raise FactorError(func=func, message=f"认证失败(HTTP 401):{err}",
                          reason="令牌缺失、过期或非法,JWT 中间件返回了真实 401",
                          hint="请重新运行 login 获取新令牌,并以 --token 传入")
    try:
        body = resp.json()
    except ValueError:
        raise FactorError(func=func, message=f"响应不是 JSON(HTTP {resp.status_code})",
                          reason="可能调用了不存在的路由,或服务返回了非 JSON 内容",
                          hint="请核对接口路径与参数后重试")
    if not isinstance(body, dict):
        return body
    if resp.status_code >= 400 and "code" not in body:
        msg = body.get("error") or body.get("msg") or f"HTTP {resp.status_code}"
        raise FactorError(func=func, message=f"请求失败(HTTP {resp.status_code}):{msg}",
                          reason="服务端返回了错误状态码",
                          hint="请根据错误信息修正参数后重试")
    code = body.get("code")
    if code not in (0, 200):
        msg = body.get("msg") or body.get("message") or "未知业务错误"
        raise FactorError(func=func, message=f"业务错误(code={code}):{msg}",
                          reason="服务端拒绝了本次操作,返回的业务错误码不为 0",
                          hint="请检查参数(如用户名/密码是否正确)后重试")
    return body


def _request(func: str, method: str, base: str, path: str,
             token: Optional[str] = None,
             json_body: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
    """发送请求并解析响应。"""
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    url = base.rstrip("/") + path
    try:
        resp = requests.request(method, url, headers=headers, json=json_body,
                                timeout=DEFAULT_TIMEOUT)
    except requests.RequestException as e:
        raise FactorError(func=func, message=f"网络请求失败:{e}",
                          reason="目标服务不可达(地址错误或服务未启动)",
                          hint=f"请确认服务地址与端口({base})是否正确,或服务是否已启动")
    return _parse(func, resp)


# ---------- 命令 ----------
def cmd_login(args) -> None:
    """登录:输出完整 JSON(含 data.access_token),不保存任何文件。"""
    _check_required("login", {"name": args.name, "password": args.password},
                    {"name": "用户名", "password": "密码"})
    body = _request("login", "POST", DEFAULT_AUTH_BASE, "/auth/login",
                    json_body={"name": args.name, "password": args.password})
    data = body.get("data")
    if not data or not data.get("access_token"):
        raise FactorError(func="login", message="登录响应缺少 access_token",
                          reason="服务端未按预期返回令牌对象",
                          hint="请检查用户名/密码是否正确")
    _print(body)  # 整个响应原样输出,LLM 从 data.access_token 取令牌


def _account_url(token: str, path: str = "") -> str:
    return f"/accounts/{_user_id(token)}{path}"


def cmd_me(args) -> None:
    """读取当前用户资料。"""
    body = _request("读取当前用户资料", "GET", DEFAULT_ACCOUNT_BASE,
                    _account_url(args.token), token=args.token)
    _print(body)


def cmd_account_update(args) -> None:
    """更新当前用户资料(只传要改的字段,内部先查后写)。"""
    func = "更新当前用户资料"
    data = _parse_kv(args.kv)

    if "user_name" in data or "avatar_url" in data:
        raise FactorError(func=func, message="user_name/avatar_url 不可通过本接口修改",
                          reason="服务端在 PUT /accounts/:id 中不更新 user_name 与 avatar_url",
                          hint="user_name 创建后不可改;头像请使用 avatar 命令")

    if "real_name" in data and not str(data["real_name"]).strip():
        raise FactorError(func=func, message="real_name 为空",
                          reason="真实姓名不允许为空",
                          hint="请提供非空的 real_name")
    if "email" in data:
        _check_email(func, data["email"])
    if "mobile" in data:
        _check_phone(func, data["mobile"])

    # 先查后写:合并现有资料,避免整单覆盖清空未提供的字段
    current = _request(func, "GET", DEFAULT_ACCOUNT_BASE, _account_url(args.token),
                       token=args.token).get("data") or {}
    merged: Dict[str, Any] = {k: current.get(k) for k in ACCOUNT_FIELDS if k in current}
    merged.update(data)
    body = _request(func, "PUT", DEFAULT_ACCOUNT_BASE, _account_url(args.token),
                    json_body=merged, token=args.token)
    _print(body)


def cmd_avatar(args) -> None:
    """更新当前用户头像(请求体字段为驼峰 avatarUrl)。"""
    func = "更新头像"
    if not args.url:
        raise FactorError(func=func, message="头像地址为空",
                          reason="头像地址不允许为空",
                          hint="请提供有效的头像图片 URL")
    body = _request(func, "PUT", DEFAULT_ACCOUNT_BASE, _account_url(args.token, "/avatar"),
                    json_body={"avatarUrl": args.url}, token=args.token)
    _print(body)


def main() -> None:
    parser = argparse.ArgumentParser(prog="python scripts/cli.py",
                                     description="授权与账号:登录获取令牌、读写当前用户资料")
    parser.add_argument("--token", help="访问令牌(login 输出中的 data.access_token)")
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("login", help="登录,输出完整 JSON(含 data.access_token)")
    p.add_argument("name")
    p.add_argument("password")
    p.set_defaults(handler=cmd_login)

    p = sub.add_parser("me", help="读取当前用户资料(需 --token)")
    p.set_defaults(handler=cmd_me)

    p = sub.add_parser("account-update", help="更新当前用户资料(需 --token,传 字段=值)")
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_account_update)

    p = sub.add_parser("avatar", help="更新当前用户头像(需 --token)")
    p.add_argument("url")
    p.set_defaults(handler=cmd_avatar)

    args = parser.parse_args()

    # 除 login 外,其它命令必须提供 --token
    if args.command != "login" and not args.token:
        parser.error(f"命令 {args.command} 需要 --token 参数(请先运行 login 获取访问令牌)")

    try:
        args.handler(args)
    except FactorError as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
