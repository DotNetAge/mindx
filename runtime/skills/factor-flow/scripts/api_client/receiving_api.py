"""收货入库模块(factor-flow 业务之一):收货单查询。

收货单(ReceiptNote)由采购单「已同意」后经 receiving-service 自动生成
(采购单号 CG... 对应收货单号 JC...,多批次收货时后缀 -01/-02 递增)。
本模块当前只提供**查询**能力,用于追踪采购单到货情况;收货操作
(准备/开始/确认/取消)由 WebUI 或后续模块提供。

与采购单的关联方式(重要):
- 收货单 `document_type` = "PurchaseOrder",`document_id` = 采购单 ID;
- 采购单详情的 `receipt_notes` 数组直接存有关联收货单 ID,追踪时用它反查。
"""
from __future__ import annotations

from typing import Any, Dict, List

from .core import FactorClient, FactorError

# 收货单状态枚举(与后端 ReceiptStatus 一致)
RECEIPT_STATUS = {
    1: "草稿/准备中",
    2: "待收货",
    3: "已完成",
    4: "已取消",
}


def receipt_status_name(status: int) -> str:
    """把收货单状态数值转为中文名。"""
    return RECEIPT_STATUS.get(status, f"未知({status})")


class ReceivingApi:
    """收货单操作门面(当前只读:查询)。"""

    def __init__(self, client: FactorClient):
        self.c = client

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
        """收货单列表:GET /receipt-notes/。

        支持过滤字段:order_no、warehouse_name、warehouse_id、source_warehouse_id、document_type、status。
        status:1 草稿/准备中、2 待收货、3 已完成、4 已取消。
        """
        allowed = ("order_no", "warehouse_name", "warehouse_id",
                   "source_warehouse_id", "document_type", "status")
        params: Dict[str, Any] = {"page": page, "size": size}
        for k, v in filters.items():
            if k not in allowed:
                raise FactorError(
                    func="收货单.list", message=f"不支持的过滤字段:{k}",
                    reason="收货单查询只支持过滤字段:" + "、".join(allowed),
                    hint="请从支持的过滤字段中选择,或去掉该参数",
                )
            if v is None or v == "":
                continue
            params[k] = v
        body = self.c.request("收货单.list", "GET", self.c.receiving_base, "/receipt-notes/", params=params)
        return self._rows(body, page, size)

    def get(self, id: str) -> Dict[str, Any]:
        """按 ID 查询收货单详情:GET /receipt-notes/:id(含 items 明细)。"""
        if not id:
            raise FactorError(func="收货单.get", message="缺少 ID 参数",
                              reason="查询详情必须提供收货单 ID",
                              hint="请先通过 po-track 或 rn-list 查询获取收货单 ID")
        body = self.c.request("收货单.get", "GET", self.c.receiving_base, f"/receipt-notes/{id}")
        return body.get("data") or body

    def find_for(self, ids: List[str]) -> List[Dict[str, Any]]:
        """按 ID 批量查询:GET /receipt-notes/find-for?ids=a,b。"""
        if not ids:
            raise FactorError(func="收货单.find_for", message="缺少 ID 列表",
                              reason="批量查询必须提供至少一个收货单 ID",
                              hint="请提供逗号分隔的收货单 ID 列表")
        body = self.c.request("收货单.find_for", "GET", self.c.receiving_base,
                              "/receipt-notes/find-for", params={"ids": ",".join(ids)})
        data = body.get("data")
        return data if isinstance(data, list) else (data or [])
