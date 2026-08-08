"""调拨模块(factor-flow 业务之一):调拨单查询。

调拨单(TransferOrder)在仓库间转移物料,状态流转:
待审核 → 已同意 → 调出完成 → 调入开始 → 已完成;可拒绝/取消。
调出出库单完成 → 调出完成;调入入库单创建/完成 → 调入开始/已完成。

本模块当前只提供查询;调拨操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 调拨单状态枚举(与后端 TransferStatus 一致)
TRANSFER_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "调出完成",
    4: "调入开始",
    5: "已完成",
    6: "已拒绝",
    7: "已取消",
}

# 调拨类型枚举(与后端 TransferType 一致)
TRANSFER_TYPE = {
    0: "正常调拨",
    1: "紧急调拨",
    2: "库存平衡",
    3: "生产需求",
}


class TransferApi(BizQueryApi):
    """调拨单查询门面。"""

    path = "/transfer-orders"
    base = "transfer_base"
    # 仅保留服务端实际生效的过滤字段;type/created_at_start/created_at_end 在服务端已注释未生效,不放入白名单
    filters = ("order_no", "source_warehouse_name", "source_warehouse_id",
               "target_warehouse_name", "target_warehouse_id",
               "status", "creator_name")
    status_enum = TRANSFER_STATUS
