"""维修模块(factor-flow 业务之一):维修单查询。

维修单(RepairOrder)记录不良品/故障品返修,状态流转:
待审核 → 已同意 → 维修中 → 已完成;可拒绝/取消。

本模块当前只提供查询;维修操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 维修单状态枚举(与后端 RepairStatus 一致;1 的中文含义以审核流程为准)
REPAIR_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "维修中",
    4: "已完成",
    5: "已拒绝",
    6: "已取消",
}


class RepairingApi(BizQueryApi):
    """维修单查询门面。"""

    path = "/repair-orders"
    base = "repairing_base"
    filters = ("order_no", "warehouse_id", "warehouse_name", "status")
    status_enum = REPAIR_STATUS
