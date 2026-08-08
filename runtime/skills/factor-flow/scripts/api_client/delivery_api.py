"""发货模块(factor-flow 业务之一):发货单(出库单)查询。

发货单(DeliveryNote)由销售订单发货或调拨调出等场景产生,状态流转:
待审核 → 已同意 → 已完成/部分发货;可拒绝/取消。
多批次发货时单号后缀 -01/-02 递增(与采购收货单规则一致)。

发货单「已完成」事件驱动库存出库(负数)与销售订单已发货数量累加。

本模块当前只提供查询;发货操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 发货单状态枚举(与后端 DeliveryStatus 一致)
DELIVERY_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "已完成",
    4: "部分发货",
    5: "已拒绝",
    6: "已取消",
}


class DeliveryApi(BizQueryApi):
    """发货单查询门面。"""

    path = "/delivery-notes"
    base = "delivery_base"
    filters = ("order_no", "customer_name", "warehouse_name", "contact_phone",
               "contact_name", "status", "warehouse_id", "target_warehouse_id")
    status_enum = DELIVERY_STATUS
    has_find_for = True
