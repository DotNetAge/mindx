"""库存模块(factor-flow 业务之一):库存查询。

库存(stock-service)是 CQRS 读模型:收货单「已完成」事件驱动库存增加
(PushItem +Quantity),出库为负数。每次入库是一条独立库存记录。

查询方式说明(重要):
- `list`(物理库存列表):按仓库/供应商/编码/名称过滤,**不支持 item_id 精确过滤**;
- `warehouse_summary`(仓库库存摘要):按 warehouse_id + item_id 精确聚合,
  强制 Quantity > 0,按库存量降序——**核对"收货仓库某物料当前库存"用它**;
- `sku_summary`(SKU 摘要):跨仓库按物料汇总,item_id 为 LIKE 匹配;
- `supplier_summary`(供应商摘要):按供应商/仓库/物料汇总。

本模块只提供**查询**;库存盘点/调整等写操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from typing import Any, Dict

from .core import FactorClient, FactorError


class InventoryApi:
    """库存查询门面(当前只读)。"""

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

    def _query(self, func: str, path: str, allowed: tuple, page: int, size: int,
               filters: Dict[str, Any]) -> Dict[str, Any]:
        """通用分页查询:过滤字段白名单 + page/size 必传。"""
        params: Dict[str, Any] = {"page": page, "size": size}
        for k, v in filters.items():
            if k not in allowed:
                raise FactorError(
                    func=func, message=f"不支持的过滤字段:{k}",
                    reason=f"{func} 只支持过滤字段:" + "、".join(allowed),
                    hint="请从支持的过滤字段中选择,或去掉该参数",
                )
            if v is None or v == "":
                continue
            params[k] = v
        body = self.c.request(func, "GET", self.c.stock_base, path, params=params)
        return self._rows(body, page, size)

    def list(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """物理库存列表:GET /inventories/。

        过滤字段:warehouse_id、warehouse_name、supplier_id、supplier_name、code、name。
        注意:此接口**不支持 item_id 过滤**,也不支持 batch_id 过滤
        (后端 ListParams 虽声明 batch_id,但查询条件未启用);
        核对某物料库存请用 warehouse_summary。
        """
        allowed = ("warehouse_id", "warehouse_name",
                   "supplier_id", "supplier_name", "code", "name")
        return self._query("库存.list", "/inventories/", allowed, page, size, filters)

    def get(self, id: str) -> Dict[str, Any]:
        """按 ID 查询单条库存记录:GET /inventories/:id。"""
        if not id:
            raise FactorError(func="库存.get", message="缺少 ID 参数",
                              reason="查询详情必须提供库存记录 ID",
                              hint="请先通过 inv-list 查询获取库存记录 ID")
        body = self.c.request("库存.get", "GET", self.c.stock_base, f"/inventories/{id}")
        return body.get("data") or body

    def warehouse_summary(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """仓库库存摘要:GET /inventories/warehouse-summary/。

        过滤字段:warehouse_id、warehouse_name、item_id、code、name。
        支持 item_id 精确匹配,强制 Quantity > 0,按库存量降序。
        """
        allowed = ("warehouse_id", "warehouse_name", "item_id", "code", "name")
        return self._query("库存.warehouse_summary", "/inventories/warehouse-summary/",
                           allowed, page, size, filters)

    def sku_summary(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """SKU 库存摘要(跨仓库按物料汇总):GET /inventories/sku-summary/。

        过滤字段:item_id、code、name(item_id 为 LIKE 模糊匹配)。
        """
        allowed = ("item_id", "code", "name")
        return self._query("库存.sku_summary", "/inventories/sku-summary/",
                           allowed, page, size, filters)

    def supplier_summary(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """供应商库存摘要:GET /inventories/supplier-summary/。

        过滤字段:warehouse_id、warehouse_name、supplier_id、supplier_name、item_id、code、name。
        """
        allowed = ("warehouse_id", "warehouse_name", "supplier_id",
                   "supplier_name", "item_id", "code", "name")
        return self._query("库存.supplier_summary", "/inventories/supplier-summary/",
                           allowed, page, size, filters)

    def stock_of(self, warehouse_id: str, item_id: str) -> Dict[str, Any]:
        """查询某仓库某物料的当前库存合计(供追踪核对使用)。

        用 warehouse-summary 按 warehouse_id + item_id 精确聚合,合计各行数量。
        无库存记录时返回 0 并标记 stock_found=False;查询失败时容错返回 None。
        """
        try:
            result = self.warehouse_summary(warehouse_id=warehouse_id, item_id=item_id)
        except FactorError as e:
            return {"stock_qty": None, "stock_found": False, "error": str(e)}
        rows = result.get("data") or []
        total = sum(float(r.get("quantity") or 0) for r in rows)
        return {
            "stock_qty": round(total, 3),
            "stock_found": len(rows) > 0,
            "records": rows,
        }
