"""生产模块(factor-flow 业务之一):生产工单查询。

生产工单(ProductionOrder)按成品 BOM 组织生产,状态流转:
待处理 → 已接单 → 生产中 → 已完成;可挂起/拒绝/取消。

本模块当前只提供查询;生产下单/领料等写操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 生产工单状态枚举(与后端 OrderStatus 一致)
PRODUCTION_STATUS = {
    1: "待处理",
    2: "已接单",
    3: "生产中",
    4: "已完成",
    5: "已挂起",
    6: "已拒绝",
    7: "已取消",
}

# 工单类型枚举(与后端 OrderType 一致)
PRODUCTION_ORDER_TYPE = {
    0: "加工",
    1: "包装",
    2: "维修",
    3: "其他",
}


class ProductionApi(BizQueryApi):
    """生产工单查询门面。"""

    path = "/production-orders"
    base = "production_base"
    # 仅保留服务端实际生效的过滤字段;accepted_at 在服务端已注释未生效,不放入白名单
    filters = ("order_type", "status", "warehouse_name", "name",
               "handler_name", "warehouse_id", "target_warehouse_id")
    status_enum = PRODUCTION_STATUS
    has_find_for = True
