"""退货模块(factor-flow 业务之一):退货单查询。

退货单(ReturnOrder)记录客户退回的物料(来源多为销售退货),状态流转:
待审核 → 已同意 → 已完成;可拒绝/取消。
退货完成通常驱动库存/报废等后续处理。

本模块当前只提供查询;退货操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 退货单状态枚举(与后端 ReturnStatus 一致)
RETURN_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "已完成",
    4: "已拒绝",
    5: "已取消",
}

# 退货类型枚举(与后端 ReturnType 一致)
RETURN_TYPE = {
    0: "质量问题",
    1: "数量不符",
    2: "包装损坏",
    3: "发错货",
    4: "滞销",
    5: "换货",
}


class ReturningApi(BizQueryApi):
    """退货单查询门面。"""

    path = "/return-orders"
    base = "returning_base"
    # 仅保留服务端实际生效的过滤字段;return_type/return_date 在服务端已注释未生效,不放入白名单
    filters = ("order_no", "warehouse_name", "customer_name", "creator_name",
               "qc_name", "status")
    status_enum = RETURN_STATUS
