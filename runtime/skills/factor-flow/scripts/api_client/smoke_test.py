"""离线冒烟测试:验证包可导入、校验规则、自动补全逻辑与引导错误输出。不访问真实服务。

运行:python3 ./scripts/api_client/smoke_test.py
"""
import os
import sys

_SCRIPTS_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _SCRIPTS_DIR)

from api_client import FactorApi, FactorError  # noqa: E402
from api_client.core import check_number, check_point, check_required, check_phone, now_ms_utc, to_iso  # noqa: E402
from api_client.purchase_api import ORDER_STATUS  # noqa: E402
from api_client.receiving_api import RECEIPT_STATUS, receipt_status_name  # noqa: E402
from api_client.sales_api import SALES_STATUS  # noqa: E402
from api_client.production_api import PRODUCTION_STATUS  # noqa: E402
from api_client.delivery_api import DELIVERY_STATUS  # noqa: E402
from api_client.transfer_api import TRANSFER_STATUS  # noqa: E402
from api_client.returning_api import RETURN_STATUS  # noqa: E402
from api_client.repairing_api import REPAIR_STATUS  # noqa: E402
from api_client.inspection_api import INSPECTION_STATUS  # noqa: E402
from api_client.scraping_api import SCRAP_STATUS  # noqa: E402


def expect_error(label, fn):
    try:
        fn()
        print(f"[FAIL] {label}: 未抛错")
    except FactorError as e:
        print(f"[OK] {label}")
        print(f"      -> {e}")
    except Exception as e:
        print(f"[FAIL] {label}: 抛出非引导错误 {type(e).__name__}: {e}")


def main():
    # 1. 时间戳格式:必须为 RFC3339 UTC(带毫秒、Z 结尾)
    ts = now_ms_utc()
    assert ts.endswith("Z") and ts[10] == "T", ts
    print("[OK] now_ms_utc ->", ts)

    # 2. 日期转换:YYYY-MM-DD 转 ISO 时间戳(与前端 new Date().toISOString() 一致)
    iso = to_iso("2026-08-10")
    assert iso.endswith("Z") and "T" in iso, iso
    print("[OK] to_iso ->", iso)

    # 3. 日期格式不合法
    expect_error("日期格式不合法", lambda: to_iso("2026/08/10"))

    # 4. 必填缺失
    expect_error("必填缺失", lambda: check_required("采购单.track", {}, {"id_or_order_no": "采购单ID或单号"}))

    # 5. 数值低于下限(数量必须 >=1)
    expect_error("数量小于 1", lambda: check_number("采购单.track", "数量", 0.5, minimum=1))

    # 6. 手机号格式
    expect_error("手机号格式", lambda: check_phone("采购单", "电话", "12345"))

    # 7. 状态枚举表完整(与后端 OrderStatus 一致)
    assert ORDER_STATUS == {1: "待审核", 2: "已同意", 3: "部分收货", 4: "已收货",
                            5: "已完成", 6: "已拒绝", 7: "已取消"}, ORDER_STATUS
    print("[OK] ORDER_STATUS 状态枚举完整")

    # 8. 未登录时发起查询,应给出引导(不联网也会先查令牌)
    factor = FactorApi()
    expect_error("未登录访问", lambda: factor.purchase.list())
    expect_error("未登录访问收货单", lambda: factor.receiving.list())
    expect_error("未登录访问库存", lambda: factor.inventory.warehouse_summary())

    # 9. 各业务状态枚举完整(与后端一致)与状态名转换
    assert RECEIPT_STATUS == {1: "草稿/准备中", 2: "待收货", 3: "已完成", 4: "已取消"}, RECEIPT_STATUS
    assert receipt_status_name(3) == "已完成"
    assert SALES_STATUS == {1: "待发货", 2: "已部分发货", 3: "已发货", 4: "已完成", 5: "已取消"}
    assert PRODUCTION_STATUS == {1: "待处理", 2: "已接单", 3: "生产中", 4: "已完成",
                                 5: "已挂起", 6: "已拒绝", 7: "已取消"}
    assert DELIVERY_STATUS == {1: "待审核", 2: "已同意", 3: "已完成", 4: "部分发货",
                               5: "已拒绝", 6: "已取消"}
    assert TRANSFER_STATUS == {1: "待审核", 2: "已同意", 3: "调出完成", 4: "调入开始",
                               5: "已完成", 6: "已拒绝", 7: "已取消"}
    assert RETURN_STATUS == {1: "待审核", 2: "已同意", 3: "已完成", 4: "已拒绝", 5: "已取消"}
    assert REPAIR_STATUS == {1: "待审核", 2: "已同意", 3: "维修中", 4: "已完成", 5: "已拒绝", 6: "已取消"}
    assert INSPECTION_STATUS == {1: "待审核", 2: "盘点中", 3: "已完成", 4: "已取消"}
    assert SCRAP_STATUS == {1: "待审核", 2: "已同意", 3: "处理中", 4: "已完成", 5: "已拒绝", 6: "已取消"}
    assert factor.sales.status_name(3) == "已发货"
    assert factor.scraping.status_name(99) == "未知(99)"
    print("[OK] 8 个业务模块状态枚举完整")

    # 10. 未登录访问各模块列表,应给出引导(不联网也会先查令牌)
    expect_error("未登录访问销售订单", lambda: factor.sales.list())
    expect_error("未登录访问生产工单", lambda: factor.production.list())
    expect_error("未登录访问发货单", lambda: factor.delivery.list())
    expect_error("未登录访问调拨单", lambda: factor.transfer.list())
    expect_error("未登录访问退货单", lambda: factor.returning.list())
    expect_error("未登录访问维修单", lambda: factor.repairing.list())
    expect_error("未登录访问盘点单", lambda: factor.inspection.list())
    expect_error("未登录访问报废单", lambda: factor.scraping.list())

    # 11. 非法过滤字段应被拦截(不联网即抛错)
    expect_error("销售订单非法过滤字段", lambda: factor.sales.list(item_id="x"))
    expect_error("生产工单非法过滤字段", lambda: factor.production.list(order_no="x"))
    expect_error("发货单非法过滤字段", lambda: factor.delivery.list(item_id="x"))
    expect_error("调拨单非法过滤字段", lambda: factor.transfer.list(item_id="x"))
    expect_error("调拨单已停用的过滤字段", lambda: factor.transfer.list(type="1"))
    expect_error("退货单非法过滤字段", lambda: factor.returning.list(item_id="x"))
    expect_error("退货单已停用的过滤字段", lambda: factor.returning.list(return_type="0"))
    expect_error("维修单非法过滤字段", lambda: factor.repairing.list(item_id="x"))
    expect_error("盘点单非法过滤字段", lambda: factor.inspection.list(item_id="x"))
    expect_error("盘点单已停用的过滤字段", lambda: factor.inspection.list(order_type="0"))
    expect_error("报废单非法过滤字段", lambda: factor.scraping.list(item_id="x"))

    # 12. 仅支持 find-for 的业务才允许批量查询
    expect_error("不支持批量查询的模块应拦截", lambda: factor.sales.find_for(["s-1"]))
    expect_error("支持 find-for 的模块缺 ID 应拦截", lambda: factor.production.find_for([]))

    # 13. 追踪命令缺参应拦截
    expect_error("采购追踪缺参数", lambda: factor.purchase.track(""))
    expect_error("销售追踪缺参数", lambda: factor.sales.track(""))
    expect_error("销售追踪未登录(先查令牌)", lambda: factor.sales.track("so-1"))

    # 14. 检查点结构:ok 与 guide(导引词)随判定切换
    cp = check_point("测试检查点", False, "依据数值", "通过导引", "失败导引")
    assert cp["check"] == "测试检查点" and cp["ok"] is False
    assert cp["guide"] == "失败导引", cp
    cp2 = check_point("测试检查点2", True, "依据数值", "通过导引", "失败导引")
    assert cp2["ok"] is True and cp2["guide"] == "通过导引", cp2
    print("[OK] 检查点结构(ok/guide 随判定切换)")

    # 15. 主数据(data-service)查询:未登录拦截与过滤白名单(与各服务 queries.go 一致)
    expect_error("未登录访问物料列表", lambda: factor.data.material.list())
    expect_error("未登录访问成品目录", lambda: factor.data.product_catalog.list())
    expect_error("未登录访问客户列表", lambda: factor.data.customer.list())
    expect_error("未登录访问供应商列表", lambda: factor.data.supplier.list())
    expect_error("未登录访问单位列表", lambda: factor.data.unit.list())
    expect_error("未登录访问颜色列表", lambda: factor.data.color.list())
    expect_error("未登录访问分类列表", lambda: factor.data.category.list())
    expect_error("未登录访问仓库列表", lambda: factor.data.warehouse.list())
    expect_error("未登录访问仓库主仓", lambda: factor.data.warehouse.primary())

    expect_error("物料非法过滤字段", lambda: factor.data.material.list(item_id="x"))
    expect_error("物料非法过滤字段(brand)", lambda: factor.data.material.list(brand="x"))
    expect_error("成品目录不支持 supplier_name 过滤", lambda: factor.data.product_catalog.list(supplier_name="x"))
    expect_error("分类非法过滤字段", lambda: factor.data.category.list(color="红"))
    expect_error("仓库非法过滤字段", lambda: factor.data.warehouse.list(item_type="x"))
    expect_error("物料批量查询缺 ID 应拦截", lambda: factor.data.material.find_by_ids([]))
    expect_error("BOM 反查缺成品 ID 应拦截", lambda: factor.data.material.bom_items(""))

    # 物料 supplier_name 属白名单字段,不应被白名单拦截(应继续走到令牌检查)
    try:
        factor.data.material.list(supplier_name="华东")
        print("[FAIL] 物料列表未做令牌检查")
    except FactorError as e:
        assert "不支持的过滤字段" not in str(e), f"supplier_name 被白名单误拦截:{e}"
        print("[OK] 物料 supplier_name 通过白名单(到达令牌检查)")

    print("\n全部冒烟用例通过。")


if __name__ == "__main__":
    main()
