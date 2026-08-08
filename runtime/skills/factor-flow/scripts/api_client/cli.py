"""Factor 业务流(factor-flow)命令行入口(供 Agent 以 `python <本文件> <命令>` 方式调用)。

本脚本聚合 factor-flow 全部查询与追踪命令,命令以模块前缀区分:
    po-*   采购单(purchase order)
    rn-*   收货单(receipt note)
    inv-*  库存(inventory)
    so-*   销售订单(sales order)
    prd-*  生产工单(production order)
    dn-*   发货单(delivery note)
    tr-*   调拨单(transfer order)
    ro-*   退货单(return order)
    rp-*   维修单(repair order)
    ins-*  盘点单(inspection/inventory check)
    sc-*   报废单(scrap order)
    <实体>-*  主数据查询(category/color/customer/material/catalog/supplier/unit/warehouse)

本技能只提供查询与追踪,不含任何写操作;下单/审核/收货/发货等均由 WebUI 完成。
主数据维护(创建/更新/删除/报价/BOM 等)由「数据管理」技能完成。

用法(在技能目录下执行):
    python ./scripts/api_client/cli.py --token <访问令牌> po-list --size 20
    python ./scripts/api_client/cli.py --token <访问令牌> po-track <采购单ID或CG单号>
    python ./scripts/api_client/cli.py --token <访问令牌> so-list status=3 --size 20
    python ./scripts/api_client/cli.py --token <访问令牌> supplier-list name=华东 --size 20
    python ./scripts/api_client/cli.py --token <访问令牌> material-find <物料ID,物料ID>

访问令牌获取:先通过「授权与账号」技能登录,从输出 JSON 取 data.access_token
暂存于对话上下文,再以 --token 传入。本脚本不保存任何令牌文件。

所有命令的完整说明见本技能 references/usage.md 与 references/modules/。
错误统一按「问题→原因→下一步」输出引导提示,退出码 1。
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from functools import partial

# 让脚本能从任意工作目录导入 api_client 包
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from api_client import FactorApi, FactorError  # noqa: E402


def _print(obj) -> None:
    """完整输出 JSON(不截断),保证关键字段原样呈现。"""
    print(json.dumps(obj, ensure_ascii=False, indent=2, default=str))


def _convert(s: str):
    """把命令行字符串转为最合理的类型(int/float/bool/str)。"""
    if s in ("true", "false"):
        return s == "true"
    try:
        return int(s)
    except ValueError:
        pass
    try:
        return float(s)
    except ValueError:
        pass
    return s


def _parse_kv(pairs) -> dict:
    """解析 字段=值 参数列表为字典。"""
    data = {}
    for p in pairs:
        if "=" not in p:
            raise FactorError(func="参数解析", message=f"参数格式应为 字段=值,收到:{p}",
                              reason="命令行参数缺少等号", hint="请使用 字段=值 形式传参")
        k, v = p.split("=", 1)
        data[k] = _convert(v)
    return data


def _split_csv(s: str) -> list:
    """解析逗号分隔列表,并去除空项。"""
    return [x.strip() for x in s.split(",") if x.strip()]


# ---------- 查询 ----------
def cmd_list(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.purchase.list(page=args.page, size=args.size, **filters))


def cmd_get(args) -> None:
    _print(factor.purchase.get(args.id))


def cmd_counting(args) -> None:
    _print({"status": args.status, "count": factor.purchase.counting(args.status)})


def cmd_find_for(args) -> None:
    _print(factor.purchase.find_for(_split_csv(args.ids)))


def cmd_export(args) -> None:
    _print(factor.purchase.export(args.id, save_path=args.out))


# ---------- 追踪 ----------
def cmd_track(args) -> None:
    _print(factor.purchase.track(args.id_or_order_no))


def cmd_so_track(args) -> None:
    _print(factor.sales.track(args.id_or_order_no))


# ---------- 收货单(rn-) ----------
def cmd_rn_list(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.receiving.list(page=args.page, size=args.size, **filters))


def cmd_rn_get(args) -> None:
    _print(factor.receiving.get(args.id))


def cmd_rn_find_for(args) -> None:
    _print(factor.receiving.find_for(_split_csv(args.ids)))


# ---------- 库存(inv-) ----------
def cmd_inv_list(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.inventory.list(page=args.page, size=args.size, **filters))


def cmd_inv_get(args) -> None:
    _print(factor.inventory.get(args.id))


def cmd_inv_warehouse_summary(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.inventory.warehouse_summary(page=args.page, size=args.size, **filters))


def cmd_inv_sku_summary(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.inventory.sku_summary(page=args.page, size=args.size, **filters))


def cmd_inv_supplier_summary(args) -> None:
    filters = _parse_kv(args.kv)
    _print(factor.inventory.supplier_summary(page=args.page, size=args.size, **filters))


# ---------- 其他业务模块查询(so-/prd-/dn-/tr-/ro-/rp-/ins-/sc-,统一模式) ----------
def cmd_biz_list(api, args) -> None:
    filters = _parse_kv(args.kv)
    _print(api.list(page=args.page, size=args.size, **filters))


def cmd_biz_get(api, args) -> None:
    _print(api.get(args.id))


def cmd_biz_find_for(api, args) -> None:
    _print(api.find_for(_split_csv(args.ids)))


def _register_biz_queries(sub, prefix: str, api, title: str, find_for: bool = False) -> None:
    """注册某业务模块的 list / get(/find-for) 查询子命令(so-/prd-/dn-/tr-/ro-/rp-/ins-/sc-)。"""
    p = sub.add_parser(f"{prefix}-list", help=f"{title}列表(过滤字段=值,详见 references/modules 文档)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=partial(cmd_biz_list, api))

    p = sub.add_parser(f"{prefix}-get", help=f"{title}详情")
    p.add_argument("id")
    p.set_defaults(handler=partial(cmd_biz_get, api))

    if find_for:
        p = sub.add_parser(f"{prefix}-find-for", help=f"{title}按 ID 批量查询(逗号分隔)")
        p.add_argument("ids")
        p.set_defaults(handler=partial(cmd_biz_find_for, api))


# data-service 实体查询命令(命名与「数据管理」技能一致,便于 Agent 复用)
DATA_ENTITIES = (
    ("category", "category", "分类"),
    ("color", "color", "颜色"),
    ("customer", "customer", "客户"),
    ("material", "material", "物料"),
    ("catalog", "product_catalog", "成品目录"),
    ("supplier", "supplier", "供应商"),
    ("unit", "unit", "计量单位"),
    ("warehouse", "warehouse", "仓库"),
)


def _register_data_queries(sub) -> None:
    """注册主数据实体查询子命令(list/get;物料/成品加 find-for 与 bom、仓库加 primary)。"""
    for disp, attr, title in DATA_ENTITIES:
        api = getattr(factor.data, attr)
        p = sub.add_parser(f"{disp}-list", help=f"{title}列表(过滤字段=值)")
        p.add_argument("--page", type=int, default=1)
        p.add_argument("--size", type=int, default=10)
        p.add_argument("kv", nargs="*")
        p.set_defaults(handler=partial(cmd_biz_list, api))

        p = sub.add_parser(f"{disp}-get", help=f"{title}详情")
        p.add_argument("id")
        p.set_defaults(handler=partial(cmd_biz_get, api))

    p = sub.add_parser("material-find", help="物料按 ID 批量查询(逗号分隔)")
    p.add_argument("ids")
    p.set_defaults(handler=partial(cmd_biz_find_for, factor.data.material))

    p = sub.add_parser("material-bom", help="按成品 ID 反查 BOM 物料")
    p.add_argument("catalog_id")
    p.set_defaults(handler=lambda args: _print(factor.data.material.bom_items(args.catalog_id)))

    p = sub.add_parser("catalog-find", help="成品目录按 ID 批量查询(逗号分隔)")
    p.add_argument("ids")
    p.set_defaults(handler=partial(cmd_biz_find_for, factor.data.product_catalog))

    p = sub.add_parser("warehouse-primary", help="查询主仓")
    p.set_defaults(handler=lambda args: _print(factor.data.warehouse.primary()))


def main() -> None:
    parser = argparse.ArgumentParser(prog="python scripts/api_client/cli.py",
                                     description="Factor 业务流(factor-flow)命令行入口")
    parser.add_argument("--token", help="访问令牌(通过「授权与账号」技能 login 获取)")
    sub = parser.add_subparsers(dest="command", required=True)

    # ==================== 采购单模块(po-) ====================
    # 查询
    p = sub.add_parser("po-list", help="采购单列表(支持 order_no/status/supplier_name/handler_name/ordered_at/warehouse_id 过滤)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_list)

    p = sub.add_parser("po-get", help="采购单详情")
    p.add_argument("id")
    p.set_defaults(handler=cmd_get)

    p = sub.add_parser("po-counting", help="采购单按状态计数(1待审核/2已同意/3部分收货/4已收货/5已完成/6已拒绝/7已取消)")
    p.add_argument("--status", type=int, required=True)
    p.set_defaults(handler=cmd_counting)

    p = sub.add_parser("po-find-for", help="采购单按 ID 批量查询(逗号分隔)")
    p.add_argument("ids")
    p.set_defaults(handler=cmd_find_for)

    p = sub.add_parser("po-export", help="导出单个采购单 Excel")
    p.add_argument("id")
    p.add_argument("--out", default=None, help="保存路径,省略则只输出字节数")
    p.set_defaults(handler=cmd_export)

    # ==================== 追踪 ====================
    p = sub.add_parser("po-track", help="按采购单ID或CG单号追踪到货情况(收货单+库存核对+剩余应到货量)")
    p.add_argument("id_or_order_no", help="采购单 ID,或以 CG 开头的采购单号")
    p.set_defaults(handler=cmd_track)

    # ==================== 收货单模块(rn-) ====================
    # 查询
    p = sub.add_parser("rn-list", help="收货单列表(支持 order_no/warehouse_name/warehouse_id/source_warehouse_id/document_type/status 过滤)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_rn_list)

    p = sub.add_parser("rn-get", help="收货单详情(含明细 items)")
    p.add_argument("id")
    p.set_defaults(handler=cmd_rn_get)

    p = sub.add_parser("rn-find-for", help="收货单按 ID 批量查询(逗号分隔)")
    p.add_argument("ids")
    p.set_defaults(handler=cmd_rn_find_for)

    # ==================== 库存模块(inv-) ====================
    # 查询
    p = sub.add_parser("inv-list", help="物理库存列表(支持 warehouse_id/warehouse_name/supplier_id/supplier_name/code/name 过滤;不支持 item_id 与 batch_id)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_inv_list)

    p = sub.add_parser("inv-get", help="单条库存记录详情")
    p.add_argument("id")
    p.set_defaults(handler=cmd_inv_get)

    p = sub.add_parser("inv-warehouse-summary", help="仓库库存摘要(支持 warehouse_id+item_id 精确过滤;Quantity>0,按库存降序)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_inv_warehouse_summary)

    p = sub.add_parser("inv-sku-summary", help="SKU 库存摘要(跨仓库按物料汇总;item_id 为模糊匹配)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_inv_sku_summary)

    p = sub.add_parser("inv-supplier-summary", help="供应商库存摘要(支持 supplier_id/warehouse_id/item_id 等过滤)")
    p.add_argument("--page", type=int, default=1)
    p.add_argument("--size", type=int, default=10)
    p.add_argument("kv", nargs="*")
    p.set_defaults(handler=cmd_inv_supplier_summary)

    # ==================== 其他业务模块查询(so-/prd-/dn-/tr-/ro-/rp-/ins-/sc-) ====================
    _register_biz_queries(sub, "so", factor.sales, "销售订单")
    p = sub.add_parser("so-track", help="按销售订单ID或XS单号追踪发货情况(发货单+库存核对+剩余应发量)")
    p.add_argument("id_or_order_no", help="销售订单 ID,或以 XS 开头的销售单号")
    p.set_defaults(handler=cmd_so_track)
    _register_biz_queries(sub, "prd", factor.production, "生产工单", find_for=True)
    _register_biz_queries(sub, "dn", factor.delivery, "发货单", find_for=True)
    _register_biz_queries(sub, "tr", factor.transfer, "调拨单")
    _register_biz_queries(sub, "ro", factor.returning, "退货单")
    _register_biz_queries(sub, "rp", factor.repairing, "维修单")
    _register_biz_queries(sub, "ins", factor.inspection, "盘点单")
    _register_biz_queries(sub, "sc", factor.scraping, "报废单")

    # ==================== 主数据查询(data-service 实体) ====================
    _register_data_queries(sub)

    args = parser.parse_args()

    # 注入令牌:所有命令都需要认证,令牌由「授权与账号」技能 login 输出
    factor.client.set_token(args.token)

    try:
        args.handler(args)
    except FactorError as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)


# 全局客户端:令牌由 main() 以 --token 注入,不保存任何令牌文件
factor = FactorApi()

if __name__ == "__main__":
    main()
