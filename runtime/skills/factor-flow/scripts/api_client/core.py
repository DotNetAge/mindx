"""Factor 业务流(factor-flow)客户端基础层:引导性错误、HTTP 客户端、校验工具。

factor-flow 聚合平台全部业务单据流(采购、收货、库存、生产、销售等),各业务
以模块形式接入,统一复用本基础层。

设计目标:让 Agent 少填数据、多用查询。所有校验失败都抛出引导性错误
(「问题 -> 原因 -> 下一步」),而不是晦涩的异常堆栈。

服务地址:
- 采购单服务(purchasing-service):8082
- 数据服务(data-service):8081(供应商/仓库/物料/成品主数据查询)
- 账号服务(account-service):8182(当前账号信息,经手人/审核人)
- 审计服务(audit-service):8080(审批通过/拒绝)
- 收货服务(receiving-service):8086(收货单查询,追踪到货情况)
- 库存服务(stock-service):8084(库存查询,核对收货仓库库存)
- 销售服务(sales-service):8083(销售订单查询)
- 生产服务(production-service):8088(生产工单查询)
- 发货服务(delivery-service):8085(发货单查询)
- 调拨服务(transfer-service):8090(调拨单查询)
- 退货服务(returning-service):8087(退货单查询)
- 维修服务(repairing-service):8091(维修单查询)
- 盘点服务(inspection-service):8089(盘点单查询)
- 报废服务(scraping-service):8092(报废单查询)
"""
from __future__ import annotations

import json
import re
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import requests

DEFAULT_TIMEOUT = 30
DEFAULT_PURCHASE_BASE = "http://localhost:8082"
DEFAULT_DATA_BASE = "http://localhost:8081"
DEFAULT_ACCOUNT_BASE = "http://localhost:8182"
DEFAULT_AUDIT_BASE = "http://localhost:8080"
DEFAULT_RECEIVING_BASE = "http://localhost:8086"
DEFAULT_STOCK_BASE = "http://localhost:8084"
DEFAULT_SALES_BASE = "http://localhost:8083"
DEFAULT_PRODUCTION_BASE = "http://localhost:8088"
DEFAULT_DELIVERY_BASE = "http://localhost:8085"
DEFAULT_TRANSFER_BASE = "http://localhost:8090"
DEFAULT_RETURNING_BASE = "http://localhost:8087"
DEFAULT_REPAIRING_BASE = "http://localhost:8091"
DEFAULT_INSPECTION_BASE = "http://localhost:8089"
DEFAULT_SCRAPING_BASE = "http://localhost:8092"

# 常见响应状态码对应的中文说明,用于引导提示
_STATUS_MSG = {
    400: "请求参数绑定失败(参数缺失或格式错误)",
    401: "未认证或令牌失效",
    404: "目标资源不存在",
    500: "服务端错误",
}


class FactorError(Exception):
    """引导性错误:以「问题 -> 原因 -> 下一步」提示,便于 Agent 直接理解并修正。"""

    def __init__(self, func: str, message: str, reason: str, hint: str):
        self.func = func
        self.message = message
        self.reason = reason
        self.hint = hint
        super().__init__(self.__str__())

    def __str__(self) -> str:
        return (
            f"调用 {self.func} 时出现问题:{self.message}。"
            f"原因:{self.reason}。"
            f"下一步:{self.hint}"
        )


def check_point(name: str, ok: bool, detail: str, pass_guide: str, fail_guide: str) -> Dict[str, Any]:
    """检查点结果:程序只做确定性判定,并返回对应导引词,供 LLM 归因分析。

    分工模式(见 guides/tracking.md):
    - 程序负责数据步骤的确定性检查(绝对精确);
    - ok=True 时 guide 为 pass_guide:引导 LLM 可直接告知用户什么;
    - ok=False 时 guide 为 fail_guide:提示 LLM 用哪些查询命令、查哪张单来分析原因。
    """
    return {
        "check": name,
        "ok": ok,
        "detail": detail,
        "guide": pass_guide if ok else fail_guide,
    }


def now_ms_utc() -> str:
    """生成 RFC3339 UTC 时间戳(毫秒级、Z 结尾),与前端 toISOString() 输出一致。"""
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def to_iso(date_str: str, func: str = "日期转换") -> str:
    """把 YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss 转为 RFC3339 UTC 时间戳。

    前端提交需求日期时使用 new Date(...).toISOString(),此处保持同一格式。
    """
    text = str(date_str).strip()
    for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d"):
        try:
            dt = datetime.strptime(text, fmt)
            return dt.astimezone(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")
        except ValueError:
            continue
    raise FactorError(
        func=func, message=f"日期格式不合法:{date_str!r}",
        reason="支持 YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss",
        hint="请使用例如 2026-08-10 的日期格式",
    )


def today_str() -> str:
    """今天日期(YYYY-MM-DD),用于需求日期校验。"""
    return datetime.now().strftime("%Y-%m-%d")


def _missing_text(required: Dict[str, str], data: Dict[str, Any]) -> List[str]:
    """返回缺失的必填字段名(带中文标签)。"""
    return [f"{key}({label})" for key, label in required.items() if not data.get(key)]


def check_required(func: str, data: Dict[str, Any], required: Dict[str, str]) -> None:
    """必填检查:缺失时抛出引导性错误。"""
    missing = _missing_text(required, data)
    if missing:
        keys = "、".join(missing)
        raise FactorError(
            func=func,
            message=f"缺少必填字段:{keys}",
            reason=f"调用 {func} 时必填参数未提供或为空,后端无校验,缺失会直接落库产生脏数据",
            hint="请补充必填字段后重试",
        )


def check_number(func: str, field: str, value: Any, minimum: Optional[float] = None,
                 precision: Optional[int] = None) -> None:
    """数值检查:最小值与小数位精度。"""
    if value is None:
        return
    try:
        num = float(value)
    except (TypeError, ValueError):
        raise FactorError(
            func=func, message=f"字段 {field} 不是数值:{value!r}",
            reason=f"{field} 应为数字类型,当前传入的不是数字",
            hint=f"请将 {field} 改为数字后再调用",
        )
    if minimum is not None and num < minimum:
        raise FactorError(
            func=func, message=f"字段 {field} 小于下限:{num} < {minimum}",
            reason=f"{field} 不允许小于 {minimum}",
            hint=f"请把 {field} 调整为不小于 {minimum} 的值",
        )
    if precision is not None:
        s = repr(num)
        if "." in s and len(s.split(".")[1].rstrip("0")) > precision:
            raise FactorError(
                func=func, message=f"字段 {field} 小数位超过 {precision} 位",
                reason=f"{field} 最多允许 {precision} 位小数",
                hint=f"请将 {field} 四舍五入到 {precision} 位小数后再调用",
            )


def check_phone(func: str, field: str, value: Optional[str]) -> None:
    """手机号格式检查(传了才检查,中国大陆 11 位手机号)。"""
    if value is None or value == "":
        return
    if not re.match(r"^1[3-9]\d{9}$", str(value)):
        raise FactorError(
            func=func, message=f"字段 {field} 的手机号格式不合法:{value}",
            reason="手机号应为 11 位数字,以 1 开头",
            hint="请改成合法的 11 位手机号",
        )


class FactorClient:
    """统一 HTTP 客户端:携带访问令牌、发送请求、解析响应,并把错误转为引导式提示。"""

    def __init__(self, purchase_base: Optional[str] = None, data_base: Optional[str] = None,
                 account_base: Optional[str] = None, audit_base: Optional[str] = None,
                 receiving_base: Optional[str] = None, stock_base: Optional[str] = None,
                 sales_base: Optional[str] = None, production_base: Optional[str] = None,
                 delivery_base: Optional[str] = None, transfer_base: Optional[str] = None,
                 returning_base: Optional[str] = None, repairing_base: Optional[str] = None,
                 inspection_base: Optional[str] = None, scraping_base: Optional[str] = None,
                 token: Optional[str] = None, timeout: Optional[int] = None):
        self.purchase_base = (purchase_base or DEFAULT_PURCHASE_BASE).rstrip("/")
        self.data_base = (data_base or DEFAULT_DATA_BASE).rstrip("/")
        self.account_base = (account_base or DEFAULT_ACCOUNT_BASE).rstrip("/")
        self.audit_base = (audit_base or DEFAULT_AUDIT_BASE).rstrip("/")
        self.receiving_base = (receiving_base or DEFAULT_RECEIVING_BASE).rstrip("/")
        self.stock_base = (stock_base or DEFAULT_STOCK_BASE).rstrip("/")
        self.sales_base = (sales_base or DEFAULT_SALES_BASE).rstrip("/")
        self.production_base = (production_base or DEFAULT_PRODUCTION_BASE).rstrip("/")
        self.delivery_base = (delivery_base or DEFAULT_DELIVERY_BASE).rstrip("/")
        self.transfer_base = (transfer_base or DEFAULT_TRANSFER_BASE).rstrip("/")
        self.returning_base = (returning_base or DEFAULT_RETURNING_BASE).rstrip("/")
        self.repairing_base = (repairing_base or DEFAULT_REPAIRING_BASE).rstrip("/")
        self.inspection_base = (inspection_base or DEFAULT_INSPECTION_BASE).rstrip("/")
        self.scraping_base = (scraping_base or DEFAULT_SCRAPING_BASE).rstrip("/")
        self.timeout = timeout or DEFAULT_TIMEOUT
        self._token = token

    # ---------- 令牌 ----------
    @property
    def token(self) -> Optional[str]:
        return self._token

    def set_token(self, token: Optional[str]) -> None:
        self._token = token

    def has_token(self) -> bool:
        return bool(self._token)

    # ---------- 请求 ----------
    def _headers(self, with_auth: bool) -> Dict[str, str]:
        headers = {"Accept": "application/json"}
        if with_auth:
            if not self.has_token():
                raise FactorError(
                    func="请求", message="缺少访问令牌",
                    reason="目标接口需要登录认证,当前没有提供访问令牌",
                    hint="请先通过「授权与账号」技能运行 login 获取访问令牌,再以 --token <令牌> 传入",
                )
            headers["Authorization"] = f"Bearer {self._token}"
        return headers

    def _parse(self, func: str, resp: requests.Response) -> Dict[str, Any]:
        """解析 sparrow 响应:业务接口 HTTP 恒 200 按 code 判断;JWT 失败是真实 401。"""
        if resp.status_code == 401:
            err = "令牌已过期或尚未生效"
            try:
                body = resp.json()
                err = body.get("error") or err
            except ValueError:
                pass
            raise FactorError(
                func=func,
                message=f"认证失败(HTTP 401):{err}",
                reason="令牌缺失、过期或非法,JWT 中间件返回了真实 401",
                hint="请重新通过「授权与账号」技能运行 login 获取新令牌,再以 --token <令牌> 传入",
            )
        try:
            body = resp.json()
        except ValueError:
            raise FactorError(
                func=func, message=f"响应不是 JSON(HTTP {resp.status_code})",
                reason="可能调用了不存在的路由,或服务返回了非 JSON 内容",
                hint="请核对接口路径与参数;若下载文件请使用对应的文件接口",
            )
        if not isinstance(body, dict):
            return body
        if resp.status_code >= 400 and "code" not in body:
            msg = body.get("error") or body.get("msg") or _STATUS_MSG.get(resp.status_code, "未知错误")
            raise FactorError(
                func=func, message=f"请求失败(HTTP {resp.status_code}):{msg}",
                reason="服务端返回了错误状态码",
                hint="请根据错误信息修正参数后重试;若为 404 请确认资源 ID 是否正确",
            )
        code = body.get("code")
        if code not in (0, 200):  # code==0 为成功
            msg = body.get("msg") or body.get("message") or "未知业务错误"
            raise FactorError(
                func=func, message=f"业务错误(code={code}):{msg}",
                reason="服务端拒绝了本次操作,返回的业务错误码不为 0",
                hint="请根据错误信息检查请求参数(如 ID 是否存在、状态是否允许)后重试",
            )
        return body

    def request(self, func: str, method: str, base: str, path: str,
                params: Optional[Dict] = None, json_body: Optional[Dict] = None,
                data: Optional[Dict] = None, files: Optional[Dict] = None,
                with_auth: bool = True, stream: bool = False) -> Any:
        """发送请求并解析响应;stream=True 时返回原始响应(用于文件下载)。"""
        url = base + path
        try:
            resp = requests.request(
                method, url, params=params, json=json_body, data=data, files=files,
                headers=self._headers(with_auth), timeout=self.timeout, stream=stream,
            )
        except requests.RequestException as e:
            raise FactorError(
                func=func, message=f"网络请求失败:{e}",
                reason="目标服务不可达(地址错误或服务未启动)",
                hint=f"请确认服务地址与端口({base})是否正确,或服务是否已启动",
            )
        if stream:
            if resp.status_code != 200:
                self._parse(func, resp)
            return resp
        return self._parse(func, resp)
