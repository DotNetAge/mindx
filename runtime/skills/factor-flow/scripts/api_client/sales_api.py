"""销售模块(factor-flow 业务之一):销售订单查询与发货追踪。

销售订单(SalesOrder)由销售业务产生,状态流转:待发货 → 部分发货/已发货 → 已完成。
发货由发货单(delivery-note)驱动:发货单完成时销售订单明细的已发货数量累加,
全部发完置「已发货」,部分发出置「已部分发货」。

追踪(so-track)按追踪总纲(references/guides/tracking.md)实现:
起点=销售单应发量 → 环节=发货单已发量 → 终点=发货仓库库存对齐。

本模块当前只提供查询与追踪;下单/审核等写操作由 WebUI 或后续模块提供。
"""
from __future__ import annotations

from collections import defaultdict
from typing import Any, Dict, List, Optional

from .biz_query import BizQueryApi
from .core import FactorClient, FactorError, check_point, check_required
from .delivery_api import DeliveryApi
from .inventory_api import InventoryApi

# 销售订单状态枚举(与后端 OrderStatus 一致)
SALES_STATUS = {
    1: "待发货",
    2: "已部分发货",
    3: "已发货",
    4: "已完成",
    5: "已取消",
}


class SalesApi(BizQueryApi):
    """销售订单查询门面。"""

    path = "/sales-orders"
    base = "sales_base"
    filters = ("order_no", "customer_name", "handler_name",
               "receiver", "receiver_phone", "ordered_at", "status")
    status_enum = SALES_STATUS

    def __init__(self, client: FactorClient, delivery: Optional[DeliveryApi] = None,
                 inventory: Optional[InventoryApi] = None):
        super().__init__(client)
        # 追踪(track)需要读取发货单与库存,共享同门面实例
        self.delivery = delivery or DeliveryApi(client)
        self.inventory = inventory or InventoryApi(client)

    # ==================== 追踪(so-track) ====================
    def track(self, id_or_order_no: str) -> Dict[str, Any]:
        """按销售订单号(ID 或 XS 单号)追踪发货情况(链路审计)。

        追踪视图包含:
        - 销售订单基本信息与状态(含关联发货单 ID);
        - 每张关联发货单:单号/状态/发货仓库/发货日期/明细;
        - 每个物料:销售数量、已发数量、各发货单汇总、剩余应发量;
        - 发货仓库当前库存(warehouse-summary)与一致性核对:
          ① 已发数量 vs 销售数量(是否发完/部分发货/未发货);
          ② 销售单已发数量 vs 各发货单汇总数量(应一致);
          ③ 发货仓库当前库存 vs 剩余应发量(库存是否足以支撑剩余发货)。
        """
        func = "销售订单.track"
        check_required(func, {"id_or_order_no": id_or_order_no},
                       {"id_or_order_no": "销售订单ID或单号"})
        text = str(id_or_order_no).strip()
        order = self._locate_order(func, text)
        return self._track_view(func, order)

    def _locate_order(self, func: str, text: str) -> Dict[str, Any]:
        """定位销售订单:XS 前缀视为单号按 order_no 过滤,否则按 ID 查详情。"""
        if text.lower().startswith("xs"):
            body = self.c.request(func, "GET", self.c.sales_base, "/sales-orders/",
                                  params={"page": 1, "size": 20, "order_no": text})
            data = body.get("data")
            if isinstance(data, dict):
                data = data.get("list", data.get("data", []))
            for row in (data or []):
                if str(row.get("order_no")) == text:
                    return row
            raise FactorError(func=func, message=f"按单号查不到销售订单:{text}",
                              reason="系统中没有该销售单号,或单号不完整",
                              hint="请核对销售单号(前缀 XS),或改用销售订单 ID 追踪")
        return self.get(text)

    def _track_view(self, func: str, order: Dict[str, Any]) -> Dict[str, Any]:
        """组装追踪视图(容错:发货/库存查询失败不阻断,以错误信息呈现)。"""
        order_id = order.get("id") or order.get("_id")
        warehouse_id = order.get("warehouse_id")

        # ---------- 关联发货单(批量查询,失败不阻断) ----------
        delivery_ids = order.get("delivery_notes") or []
        deliveries: List[Dict[str, Any]] = []
        deliveries_error: Optional[str] = None
        if delivery_ids:
            try:
                deliveries = self.delivery.find_for(delivery_ids)
            except FactorError as e:
                deliveries_error = str(e)

        # 各发货单该物料发货量汇总 + 发货单视图
        delivered_by_item: Dict[str, float] = defaultdict(float)
        delivery_view: List[Dict[str, Any]] = []
        for d in deliveries:
            total_qty = 0.0
            d_items: List[Dict[str, Any]] = []
            for it in d.get("items") or []:
                qty = float(it.get("quantity") or 0)
                total_qty += qty
                delivered_by_item[it.get("item_id")] += qty
                d_items.append({
                    "item_id": it.get("item_id"),
                    "code": it.get("code") or "",
                    "name": it.get("name") or "",
                    "spec": it.get("spec") or "",
                    "color": it.get("color") or "",
                    "unit": it.get("unit") or "",
                    "quantity": qty,
                })
            delivery_view.append({
                "id": d.get("id"),
                "order_no": d.get("order_no"),
                "status": d.get("status"),
                "status_text": self.delivery.status_name(d.get("status")),
                "warehouse_id": d.get("warehouse_id"),
                "warehouse_name": d.get("warehouse_name"),
                "deliveried_at": str(d.get("deliveried_at") or "")[:19],
                "remarks": d.get("remarks") or "",
                "items": d_items,
                "total_quantity": round(total_qty, 3),
            })

        # ---------- 每个物料的核对表 ----------
        fully = partial = none = over = 0
        amount_mismatch: List[Dict[str, Any]] = []   # 两口径不一致的物料
        stock_short: List[Dict[str, Any]] = []       # 库存不足以支撑剩余应发的物料
        items_view: List[Dict[str, Any]] = []
        for it in order.get("items") or []:
            item_id = it.get("item_id")
            if not item_id:
                continue
            quantity = float(it.get("quantity") or 0)
            delivered = float(it.get("actual_delivered") or 0)
            deliveries_sum = delivered_by_item.get(item_id, 0.0)
            remaining = round(quantity - delivered, 3)

            # 发货状态判定
            if quantity > 0 and delivered >= quantity:
                if delivered > quantity + 1e-6:
                    over += 1
                    match = "超额发货"
                else:
                    fully += 1
                    match = "已发齐"
            elif delivered > 0:
                partial += 1
                match = "部分发货"
            else:
                none += 1
                match = "未发货"

            # 一致性核对 ②:销售单已发 vs 发货单汇总
            if abs(deliveries_sum - delivered) > 1e-6:
                amount_mismatch.append({
                    "item_id": item_id, "name": it.get("name"),
                    "order_delivered": delivered, "deliveries_sum": round(deliveries_sum, 3),
                })

            # 一致性核对 ③:发货仓库当前库存 vs 剩余应发量(终点对齐)
            stock = {"stock_qty": None, "stock_found": False, "records": [], "error": None}
            if warehouse_id:
                stock = self.inventory.stock_of(warehouse_id, item_id)
            stock_qty = stock.get("stock_qty")
            stock_ok: Optional[bool] = None
            stock_note: Optional[str] = None
            if stock_qty is not None and remaining > 1e-6:
                # 还有剩余应发:库存应能支撑剩余发货
                stock_ok = bool(stock_qty + 1e-6 >= remaining)
                if not stock_ok:
                    stock_short.append({
                        "item_id": item_id, "name": it.get("name"),
                        "remaining": remaining, "stock_qty": stock_qty,
                    })
            elif stock_qty is not None and stock_qty > 0:
                stock_note = (f"订单已全部发货,但仓库 {warehouse_id} 仍有该物料库存 {stock_qty}"
                              "(可能来自其他入库),仅供终点对照")

            # 该物料出现在每张发货单中的数量
            item_deliveries: List[Dict[str, Any]] = []
            for d in deliveries:
                for it2 in d.get("items") or []:
                    if it2.get("item_id") == item_id:
                        item_deliveries.append({
                            "delivery_note_id": d.get("id"),
                            "order_no": d.get("order_no"),
                            "status_text": self.delivery.status_name(d.get("status")),
                            "quantity": round(float(it2.get("quantity") or 0), 3),
                        })

            items_view.append({
                "item_id": item_id,
                "code": it.get("code") or "",
                "name": it.get("name") or "",
                "category": it.get("category") or "",
                "spec": it.get("spec") or "",
                "color": it.get("color") or "",
                "unit": it.get("unit") or "",
                "price": it.get("price") or 0,
                "quantity": quantity,            # 销售数量(应发)
                "delivered_qty": delivered,      # 已发数量(销售单口径)
                "remaining_qty": remaining,      # 剩余应发量
                "match": match,
                "deliveries": item_deliveries,
                "stock": {
                    "warehouse_id": warehouse_id,
                    "stock_qty": stock_qty,
                    "stock_found": stock.get("stock_found"),
                    "stock_ok": stock_ok,
                    "note": stock_note,
                    "error": stock.get("error"),
                },
            })

        # ---------- 检查点报告(分工模式:程序确定性检查,LLM 归因分析,见 guides/tracking.md) ----------
        order_no = order.get("order_no") or order_id
        status_text = SALES_STATUS.get(order.get("status"), f"未知({order.get('status')})")
        checks: List[Dict[str, Any]] = []

        # c1 订单定位与状态
        checks.append(check_point(
            "订单定位与状态", True,
            f"销售订单 {order_no} 状态:{status_text}",
            f"可直接告知用户该销售订单当前状态({status_text})。",
            "此检查点不应失败,若出现请核查单据数据。",
        ))

        # c2 发货进度(已发 vs 应发)
        progress = "、".join(
            f"{it['name']}:应发{it['quantity']} 已发{it['delivered_qty']} 剩余{it['remaining_qty']}"
            for it in items_view) or "无明细"
        checks.append(check_point(
            "发货进度(已发 vs 应发)", over == 0,
            progress,
            "发货进度与销售数量匹配,可告知用户每物料的已发/剩余量。",
            f"存在超额发货({over} 项)或数量异常,请用 dn-get 逐张核查发货单明细,"
            "确认是否重复发货或多发。",
        ))

        # c3 两口径一致性
        checks.append(check_point(
            "两口径一致性(销售单已发 vs 发货单汇总)", len(amount_mismatch) == 0,
            "全部一致" if not amount_mismatch else "、".join(
                f"{m['name']}:销售单已发{m['order_delivered']} vs 发货单汇总{m['deliveries_sum']}"
                for m in amount_mismatch),
            "销售单已发数量与发货单汇总一致,发货数据完整。",
            "两口径不一致,请用 dn-find-for 核查疑点发货单的明细与状态,"
            "再用 inv-warehouse-summary 核对发货仓库存确认出库是否生效。",
        ))

        # c4 终点对齐(发货仓库存支撑剩余应发)
        checks.append(check_point(
            "终点对齐(发货仓库存支撑剩余应发)", len(stock_short) == 0,
            "库存足以支撑剩余发货" if not stock_short else "、".join(
                f"{s['name']}:剩余应发{s['remaining']} 当前库存{s['stock_qty']}" for s in stock_short),
            "剩余应发数量有库存支撑,可告知用户可继续发货。",
            "库存不足以完成剩余发货,请查在途补货渠道(prd-list 生产中 / tr-list 在途调拨),"
            "并告知用户缺货数量。",
        ))

        # c5 发货单状态完整性
        unfinished = [d for d in deliveries if d.get("status") != 3]
        checks.append(check_point(
            "发货单状态完整性", len(unfinished) == 0,
            "全部发货单已完成" if not unfinished else "、".join(
                f"{d.get('order_no') or d.get('id')}:{self.delivery.status_name(d.get('status'))}"
                for d in unfinished),
            "所有发货单均已完成,出库流程走完。",
            "存在未完成发货单(正常在途或流程未走完),请提示用户当前发货进度,"
            "并用 dn-get 查具体发货单状态。",
        ))

        # ---------- 汇总 ----------
        total_items = len(items_view)
        conclusion = "全部发货"
        if total_items == 0:
            conclusion = "销售订单没有明细"
        elif none == total_items:
            conclusion = "尚未发货"
        elif fully == total_items and over == 0:
            conclusion = "全部发货"
        elif over > 0:
            conclusion = f"存在超额发货({over} 项)"
        else:
            conclusion = f"部分发货,已发齐 {fully} 项,仍差 {partial} 项物料待发货"

        return {
            "sales_order": {
                "id": order_id,
                "order_no": order.get("order_no"),
                "customer_id": order.get("customer_id"),
                "customer_name": order.get("customer_name"),
                "warehouse_id": warehouse_id,
                "warehouse_name": order.get("warehouse_name"),
                "status": order.get("status"),
                "status_text": status_text,
                "total_amount": order.get("total_amount"),
                "ordered_at": str(order.get("ordered_at") or "")[:19],
                "scheduled_shipping_at": str(order.get("scheduled_shipping_at") or "")[:19],
                "delivery_notes": delivery_ids,
            },
            "delivery_notes": delivery_view,
            "delivery_notes_error": deliveries_error,
            "items": items_view,
            "checks": checks,
            "summary": {
                "total_items": total_items,
                "delivered_count": fully,
                "partial_count": partial,
                "not_delivered_count": none,
                "over_count": over,
                "pass_count": sum(1 for c in checks if c["ok"]),
                "fail_count": sum(1 for c in checks if not c["ok"]),
                "conclusion": conclusion,
            },
        }
