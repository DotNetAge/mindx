"""盘点模块(factor-flow 业务之一):盘点单查询。

盘点单(InventoryCheckOrder)用于核对仓库实物与账面库存,状态流转:
待审核 → 盘点中 → 已完成;可取消。

本模块当前只提供查询;盘点操作由 WebUI 或后续模块提供。

注意:后端列表接口对 Status 过滤无"缺省守卫"(其他模块均有),
不传 status 时后端按 Status=0 过滤,结果恒空——查列表请务必带 status。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 盘点单状态枚举(与后端 InventoryCheckStatus 一致)
INSPECTION_STATUS = {
    1: "待审核",
    2: "盘点中",
    3: "已完成",
    4: "已取消",
}

# 盘点类型枚举(与后端 InventoryCheckOrderType 一致)
INSPECTION_ORDER_TYPE = {
    0: "全面盘点",
    1: "部分盘点",
    2: "循环盘点",
}


class InspectionApi(BizQueryApi):
    """盘点单查询门面。"""

    path = "/inspections"
    base = "inspection_base"
    # 仅保留服务端实际生效的过滤字段;order_type/started_at/completed_at 在服务端未使用,不放入白名单
    filters = ("order_no", "warehouse_name", "warehouse_id", "status")
    status_enum = INSPECTION_STATUS
