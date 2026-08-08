"""采购单模块(factor-flow 业务之一):查询与到货追踪。

采购单(PurchaseOrder)状态流转:待审核 → 已同意 → 部分收货/已收货 → 已完成。
收货由收货单(receipt-note)驱动:收货完成驱动采购单已收数量(received_qty)累加
与库存增加。

追踪(po-track)按追踪总纲(references/guides/tracking.md)实现:
起点=采购单应到量 → 环节=收货单已收量 → 终点=收货仓库存对齐。

本模块只提供查询与追踪;下单/审核/收货等写操作由 WebUI 完成。
"""
from __future__ import annotations

from collections import defaultdict
from typing import Any, Dict, List, Optional

from .core import FactorClient, FactorError, check_point, check_required, to_iso
from .inventory_api import InventoryApi
from .receiving_api import ReceivingApi, receipt_status_name

# 采购单状态枚举(与后端 OrderStatus 一致)
ORDER_STATUS = {
    1: "待审核",
    2: "已同意",
    3: "部分收货",
    4: "已收货",
    5: "已完成",
    6: "已拒绝",
    7: "已取消",
}


class PurchaseApi:
    """采购单查询与追踪门面。"""

    def __init__(self, client: FactorClient, receiving: Optional[ReceivingApi] = None,
                 inventory: Optional[InventoryApi] = None):
        self.c = client
        # 追踪(track)需要读取收货单与库存,共享同门面实例
        self.receiving = receiving or ReceivingApi(client)
        self.inventory = inventory or InventoryApi(client)

    # ==================== 查询 ====================
    def list(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """列表查询:GET /purchase-orders/。

        支持过滤字段:order_no、status、supplier_name、handler_name、ordered_at、warehouse_id。
        ordered_at 传 YYYY-MM-DD 或 YYYY-MM-DD HH:mm:ss,自动转 ISO 时间戳。
        """
        allowed = ("order_no", "status", "supplier_name", "handler_name", "ordered_at", "warehouse_id")
        params: Dict[str, Any] = {"page": page, "size": size}
        for k, v in filters.items():
            if k not in allowed:
                raise FactorError(
                    func="采购单.list", message=f"不支持的过滤字段:{k}",
                    reason="采购单查询只支持过滤字段:" + "、".join(allowed),
                    hint=f"请从支持的过滤字段中选择,或去掉该参数",
                )
            if v is None or v == "":
                continue
            if k == "ordered_at":
                v = to_iso(v, "采购单.list")
            params[k] = v
        body = self.c.request("采购单.list", "GET", self.c.purchase_base, "/purchase-orders/", params=params)
        data = body.get("data")
        total = body.get("total")
        if isinstance(data, dict):
            data = data.get("list", data.get("data", []))
        if total is None and isinstance(data, list):
            total = len(data)
        return {"data": data or [], "total": total, "page": page, "size": size}

    def get(self, id: str) -> Dict[str, Any]:
        """按 ID 查询详情:GET /purchase-orders/:id。"""
        if not id:
            raise FactorError(func="采购单.get", message="缺少 ID 参数",
                              reason="查询详情必须提供采购单 ID",
                              hint="请先调用 list 查询获取采购单 ID")
        body = self.c.request("采购单.get", "GET", self.c.purchase_base, f"/purchase-orders/{id}")
        return body.get("data") or body

    def counting(self, status: int) -> int:
        """按状态计数:GET /purchase-orders/counting?status=N。"""
        check_required("采购单.counting", {"status": status}, {"status": "状态"})
        body = self.c.request("采购单.counting", "GET", self.c.purchase_base,
                              "/purchase-orders/counting", params={"status": status})
        return body.get("data")

    def find_for(self, ids: List[str]) -> List[Dict[str, Any]]:
        """按 ID 批量查询:GET /purchase-orders/find-for?ids=a,b。"""
        if not ids:
            raise FactorError(func="采购单.find_for", message="缺少 ID 列表",
                              reason="批量查询必须提供至少一个采购单 ID",
                              hint="请提供逗号分隔的采购单 ID 列表")
        body = self.c.request("采购单.find_for", "GET", self.c.purchase_base,
                              "/purchase-orders/find-for", params={"ids": ",".join(ids)})
        data = body.get("data")
        return data if isinstance(data, list) else (data or [])

    def export(self, id: str, save_path: Optional[str] = None) -> Dict[str, Any]:
        """导出单个采购单 Excel:GET /purchase-orders/:id/export。

        save_path 省略时返回内容摘要,不落盘。
        """
        if not id:
            raise FactorError(func="采购单.export", message="缺少 ID 参数",
                              reason="导出必须提供采购单 ID",
                              hint="请先查询获取要导出的采购单 ID")
        resp = self.c.request("采购单.export", "GET", self.c.purchase_base,
                              f"/purchase-orders/{id}/export", stream=True)
        content = resp.content
        if save_path:
            with open(save_path, "wb") as f:
                f.write(content)
            return {"saved": save_path, "bytes": len(content)}
        return {"bytes": len(content), "hint": "未指定保存路径,内容未落盘;可加 --out <路径> 保存"}

    # ==================== 追踪(采购单 -> 收货 -> 库存) ====================
    def track(self, id_or_order_no: str) -> Dict[str, Any]:
        """按采购单号(ID 或 CG 单号)追踪到货情况。

        追踪视图包含:
        - 采购单基本信息与状态(含关联收货单 ID);
        - 每张关联收货单:单号/状态/收货仓库/收货日期/明细;
        - 每个物料:采购量、已收货量、剩余应到货量;
        - 收货仓库当前库存(warehouse-summary)与一致性核对:
          ① 已收货量 vs 采购量(是否到齐/部分到货/未到货);
          ② 采购单已收数量 vs 各收货单汇总数量(应一致);
          ③ 收货仓库当前库存 vs 已收货量(库存是否覆盖已收,覆盖视为正常;
             小于已收说明物料已被领用/出库,属于异常)。
        """
        func = "采购单.track"
        check_required(func, {"id_or_order_no": id_or_order_no},
                       {"id_or_order_no": "采购单ID或单号"})
        text = str(id_or_order_no).strip()
        order = self._locate_order(func, text)
        return self._track_view(func, order)

    def _locate_order(self, func: str, text: str) -> Dict[str, Any]:
        """定位采购单:CG 前缀视为单号按 order_no 过滤,否则按 ID 查详情。"""
        if text.lower().startswith("cg"):
            body = self.c.request(func, "GET", self.c.purchase_base, "/purchase-orders/",
                                  params={"page": 1, "size": 20, "order_no": text})
            data = body.get("data")
            if isinstance(data, dict):
                data = data.get("list", data.get("data", []))
            for row in (data or []):
                if str(row.get("order_no")) == text:
                    return row
            raise FactorError(func=func, message=f"按单号查不到采购单:{text}",
                              reason="系统中没有该采购单号,或单号不完整",
                              hint="请核对采购单号(前缀 CG),或改用采购单 ID 追踪")
        return self.get(text)

    def _track_view(self, func: str, order: Dict[str, Any]) -> Dict[str, Any]:
        """组装追踪视图(容错:收货/库存查询失败不阻断,以错误信息呈现)。"""
        order_id = order.get("id") or order.get("_id")
        warehouse_id = order.get("warehouse_id")

        # ---------- 关联收货单(批量查询,失败不阻断) ----------
        receipt_ids = order.get("receipt_notes") or []
        receipts: List[Dict[str, Any]] = []
        receipts_error: Optional[str] = None
        if receipt_ids:
            try:
                receipts = self.receiving.find_for(receipt_ids)
            except FactorError as e:
                receipts_error = str(e)

        # 各收货单该物料收货量汇总 + 收货单视图
        receipts_by_item: Dict[str, float] = defaultdict(float)
        receipt_view: List[Dict[str, Any]] = []
        for r in receipts:
            total_qty = 0.0
            r_items: List[Dict[str, Any]] = []
            for it in r.get("items") or []:
                qty = float(it.get("quantity") or 0)
                total_qty += qty
                receipts_by_item[it.get("item_id")] += qty
                r_items.append({
                    "item_id": it.get("item_id"),
                    "code": it.get("code") or "",
                    "name": it.get("name") or "",
                    "spec": it.get("spec") or "",
                    "color": it.get("color") or "",
                    "unit": it.get("unit") or "",
                    "quantity": qty,
                })
            receipt_view.append({
                "id": r.get("id"),
                "order_no": r.get("order_no"),
                "status": r.get("status"),
                "status_text": receipt_status_name(r.get("status")),
                "warehouse_id": r.get("warehouse_id"),
                "warehouse_name": r.get("warehouse_name"),
                "handler_name": r.get("handler_name") or "",
                "receipted_at": str(r.get("receipted_at") or "")[:19],
                "remarks": r.get("remarks") or "",
                "items": r_items,
                "total_quantity": round(total_qty, 3),
            })

        # ---------- 每个物料的核对表 ----------
        fully = partial = none = over = 0
        amount_mismatch: List[Dict[str, Any]] = []   # 两口径不一致的物料
        stock_short: List[Dict[str, Any]] = []       # 库存未覆盖已收的物料
        items_view: List[Dict[str, Any]] = []
        for it in order.get("items") or []:
            item_id = it.get("item_id")
            if not item_id:
                continue
            quantity = float(it.get("quantity") or 0)
            received = float(it.get("received_qty") or 0)
            receipts_sum = receipts_by_item.get(item_id, 0.0)
            remaining = round(quantity - received, 3)

            # 到货状态判定
            if quantity > 0 and received >= quantity:
                if received > quantity + 1e-6:
                    over += 1
                    match = "超额收货"
                else:
                    fully += 1
                    match = "已到齐"
            elif received > 0:
                partial += 1
                match = "部分到货"
            else:
                none += 1
                match = "未到货"

            # 一致性核对 ②:采购单已收 vs 收货单汇总
            if abs(receipts_sum - received) > 1e-6:
                amount_mismatch.append({
                    "item_id": item_id, "name": it.get("name"),
                    "order_received": received, "receipts_sum": round(receipts_sum, 3),
                })

            # 一致性核对 ③:收货仓库当前库存 vs 已收货量
            stock = {"stock_qty": None, "stock_found": False, "records": [], "error": None}
            if warehouse_id:
                stock = self.inventory.stock_of(warehouse_id, item_id)
            stock_qty = stock.get("stock_qty")
            stock_ok: Optional[bool] = None
            if stock_qty is not None and received > 0:
                stock_ok = bool(stock_qty + 1e-6 >= received)
                if not stock_ok:
                    stock_short.append({
                        "item_id": item_id, "name": it.get("name"),
                        "received": received, "stock_qty": stock_qty,
                    })

            # 该物料出现在每张收货单中的数量
            item_receipts: List[Dict[str, Any]] = []
            for r in receipts:
                for it2 in r.get("items") or []:
                    if it2.get("item_id") == item_id:
                        item_receipts.append({
                            "receipt_note_id": r.get("id"),
                            "order_no": r.get("order_no"),
                            "status_text": receipt_status_name(r.get("status")),
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
                "quantity": quantity,          # 采购量
                "received_qty": received,      # 已收货量(采购单口径)
                "remaining_qty": remaining,    # 剩余应到货量
                "match": match,
                "receipts": item_receipts,
                "stock": {
                    "warehouse_id": warehouse_id,
                    "stock_qty": stock_qty,
                    "stock_found": stock.get("stock_found"),
                    "stock_ok": stock_ok,
                    "error": stock.get("error"),
                },
            })

        # ---------- 检查点报告(分工模式:程序确定性检查,LLM 归因分析,见 guides/tracking.md) ----------
        order_no = order.get("order_no") or order_id
        status_text = ORDER_STATUS.get(order.get("status"), f"未知({order.get('status')})")
        checks: List[Dict[str, Any]] = []

        # c1 订单定位与状态
        checks.append(check_point(
            "订单定位与状态", True,
            f"采购单 {order_no} 状态:{status_text}",
            f"可直接告知用户该采购单当前状态({status_text})。",
            "此检查点不应失败,若出现请核查单据数据。",
        ))

        # c2 到货进度(已到 vs 应到)
        progress = "、".join(
            f"{it['name']}:应到{it['quantity']} 已到{it['received_qty']} 剩余{it['remaining_qty']}"
            for it in items_view) or "无明细"
        checks.append(check_point(
            "到货进度(已到 vs 应到)", over == 0,
            progress,
            "到货进度与采购数量匹配,可告知用户每物料的已到/剩余量。",
            f"存在超额收货({over} 项)或数量异常,请用 rn-get 逐张核查收货单明细,"
            "确认是否重复收货或多收。",
        ))

        # c3 两口径一致性
        checks.append(check_point(
            "两口径一致性(采购单已收 vs 收货单汇总)", len(amount_mismatch) == 0,
            "全部一致" if not amount_mismatch else "、".join(
                f"{m['name']}:采购单已收{m['order_received']} vs 收货单汇总{m['receipts_sum']}"
                for m in amount_mismatch),
            "采购单已收数量与收货单汇总一致,收货数据完整。",
            "两口径不一致,请用 rn-find-for 核查疑点收货单的明细与状态,"
            "再用 inv-warehouse-summary 核对收货仓库存确认入库是否生效。",
        ))

        # c4 终点对齐(收货仓库存覆盖已收)
        checks.append(check_point(
            "终点对齐(收货仓库存覆盖已收)", len(stock_short) == 0,
            "库存已覆盖到货" if not stock_short else "、".join(
                f"{s['name']}:已收{s['received']} 当前库存{s['stock_qty']}" for s in stock_short),
            "到货已正常入账,库存覆盖已收,可告知用户到货完成。",
            "库存小于已收,可能到货未入库或已被领用/出库,请用 inv-warehouse-summary 查库存流水,"
            "并核查相关领料/发货单。",
        ))

        # c5 收货单状态完整性
        unfinished = [r for r in receipts if r.get("status") != 3]
        checks.append(check_point(
            "收货单状态完整性", len(unfinished) == 0,
            "全部收货单已完成" if not unfinished else "、".join(
                f"{r.get('order_no') or r.get('id')}:{receipt_status_name(r.get('status'))}"
                for r in unfinished),
            "所有收货单均已完成,到货流程走完。",
            "存在未完成收货单(正常在途或流程未走完),请提示用户当前收货进度,"
            "并用 rn-get 查具体收货单状态。",
        ))

        # ---------- 汇总 ----------
        total_items = len(items_view)
        conclusion = "全部到货"
        if total_items == 0:
            conclusion = "采购单没有明细"
        elif none == total_items:
            conclusion = "尚未到货"
        elif fully == total_items and over == 0:
            conclusion = "全部到货"
        elif over > 0:
            conclusion = f"存在超额收货({over} 项)"
        else:
            conclusion = f"部分到货,已到齐 {fully} 项,仍差 {partial} 项物料待收货"

        return {
            "purchase_order": {
                "id": order_id,
                "order_no": order.get("order_no"),
                "supplier_id": order.get("supplier_id"),
                "supplier_name": order.get("supplier_name"),
                "warehouse_id": warehouse_id,
                "warehouse_name": order.get("warehouse_name"),
                "status": order.get("status"),
                "status_text": status_text,
                "total_amount": order.get("total_amount"),
                "ordered_at": str(order.get("ordered_at") or "")[:19],
                "demand_date": str(order.get("demand_date") or "")[:10],
                "receipt_notes": receipt_ids,
            },
            "receipt_notes": receipt_view,
            "receipt_notes_error": receipts_error,
            "items": items_view,
            "checks": checks,
            "summary": {
                "total_items": total_items,
                "received_count": fully,
                "partial_count": partial,
                "not_received_count": none,
                "over_count": over,
                "pass_count": sum(1 for c in checks if c["ok"]),
                "fail_count": sum(1 for c in checks if not c["ok"]),
                "conclusion": conclusion,
            },
        }
