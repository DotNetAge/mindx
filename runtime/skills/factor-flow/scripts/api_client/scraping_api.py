"""报废模块(factor-flow 业务之一):报废单查询。

报废单(ScrapOrder)记录物料报废(生产损耗/质量报废/积压/过期等),状态流转:
待审核 → 已同意 → 处理中 → 已完成;可拒绝/取消。
报废完成通常驱动库存出库(负数)。

本模块当前只提供查询;报废操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from .biz_query import BizQueryApi

# 报废单状态枚举(与后端 ScrapStatus 一致)
SCRAP_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "处理中",
    4: "已完成",
    5: "已拒绝",
    6: "已取消",
}

# 报废类型枚举(与后端 ScrapType 一致)
SCRAP_TYPE = {
    0: "生产损耗",
    1: "质量报废",
    2: "积压报废",
    3: "过期报废",
}


class ScrapingApi(BizQueryApi):
    """报废单查询门面。"""

    path = "/scraps"
    base = "scraping_base"
    filters = ("order_no", "status", "warehouse_id")
    status_enum = SCRAP_STATUS
