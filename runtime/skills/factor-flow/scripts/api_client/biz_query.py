"""业务单据查询基类(factor-flow 共用)。

各业务模块(销售/生产/发货/调拨/退货/维修/盘点/报废)查询接口结构完全一致:
    GET /<path>/       列表(page/size 必传,支持过滤字段)
    GET /<path>/:id    详情
    GET /<path>/find-for?ids=  批量(部分业务支持,见 has_find_for)
本基类统一实现 list/get/find_for,子类只需声明 path、客户端地址字段、过滤字段白名单与状态枚举。
"""
from __future__ import annotations

from typing import Any, Dict, List

from .core import FactorClient, FactorError


class BizQueryApi:
    """业务单据查询基类:子类覆盖 path / base / filters / status_enum / has_find_for。"""

    path: str = ""              # HTTP 路由前缀,如 /sales-orders
    base: str = ""              # FactorClient 上的服务地址属性名,如 sales_base
    filters: tuple = ()         # 列表接口支持的过滤字段白名单
    status_enum: Dict[int, str] = {}  # 状态枚举 {数值: 中文}
    has_find_for: bool = False  # 是否支持 /find-for 批量查询

    def __init__(self, client: FactorClient):
        self.c = client

    def _base(self) -> str:
        """取子类声明的服务地址。"""
        return getattr(self.c, self.base)

    def status_name(self, status) -> str:
        """把状态数值转为中文名(未知值原样返回)。"""
        try:
            s = int(status)
        except (TypeError, ValueError):
            return f"未知({status})"
        return self.status_enum.get(s, f"未知({s})")

    def _rows(self, body: Dict[str, Any], page: int, size: int) -> Dict[str, Any]:
        """从 sparrow 分页响应中提取 data 列表与 total。"""
        data = body.get("data")
        total = body.get("total")
        if isinstance(data, dict):
            data = data.get("list", data.get("data", []))
        if total is None and isinstance(data, list):
            total = len(data)
        return {"data": data or [], "total": total, "page": page, "size": size}

    def list(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """单据列表:GET <path>/。支持子类声明的过滤字段。"""
        func = f"{self.__class__.__name__}.list"
        params: Dict[str, Any] = {"page": page, "size": size}
        for k, v in filters.items():
            if k not in self.filters:
                raise FactorError(
                    func=func, message=f"不支持的过滤字段:{k}",
                    reason=f"{self.__class__.__name__} 列表只支持过滤字段:" + "、".join(self.filters),
                    hint="请从支持的过滤字段中选择,或去掉该参数",
                )
            if v is None or v == "":
                continue
            params[k] = v
        body = self.c.request(func, "GET", self._base(), self.path + "/", params=params)
        return self._rows(body, page, size)

    def get(self, id: str) -> Dict[str, Any]:
        """按 ID 查询详情:GET <path>/:id。"""
        if not id:
            raise FactorError(func=f"{self.__class__.__name__}.get", message="缺少 ID 参数",
                              reason="查询详情必须提供单据 ID",
                              hint="请先通过列表查询获取单据 ID")
        body = self.c.request(f"{self.__class__.__name__}.get", "GET",
                              self._base(), f"{self.path}/{id}")
        return body.get("data") or body

    def find_for(self, ids: List[str]) -> List[Dict[str, Any]]:
        """按 ID 批量查询:GET <path>/find-for?ids=a,b(仅 has_find_for 的业务)。"""
        if not self.has_find_for:
            raise FactorError(func=f"{self.__class__.__name__}.find_for",
                              message="该业务不支持批量查询",
                              reason="后端未提供 /find-for 接口",
                              hint="请改用列表或详情查询")
        if not ids:
            raise FactorError(func=f"{self.__class__.__name__}.find_for", message="缺少 ID 列表",
                              reason="批量查询必须提供至少一个单据 ID",
                              hint="请提供逗号分隔的单据 ID 列表")
        body = self.c.request(f"{self.__class__.__name__}.find_for", "GET",
                              self._base(), f"{self.path}/find-for",
                              params={"ids": ",".join(ids)})
        data = body.get("data")
        return data if isinstance(data, list) else (data or [])
