"""Factor 业务流(factor-flow)API 客户端统一入口(供 Agent 以 cli.py 命令使用)。

用法(在技能目录下执行):
    python ./scripts/api_client/cli.py --token <访问令牌> po-list --size 20
    python ./scripts/api_client/cli.py --token <访问令牌> po-track <采购单ID或CG单号>
    python ./scripts/api_client/cli.py --token <访问令牌> so-list status=3 --size 20
    python ./scripts/api_client/cli.py --token <访问令牌> supplier-list name=华东 --size 20

访问令牌通过「授权与账号」技能获取(不在此保存任何文件):
    python <授权与账号技能目录>/scripts/cli.py login <用户名> <密码>
    # 从输出 JSON 的 data.access_token 取值,暂存于对话上下文,再以 --token 传入

服务地址可在构造时覆盖:
    factor = FactorApi(purchase_base="http://localhost:8082", data_base="http://localhost:8081",
                       account_base="http://localhost:8182", audit_base="http://localhost:8080",
                       receiving_base="http://localhost:8086", stock_base="http://localhost:8084",
                       sales_base="http://localhost:8083", production_base="http://localhost:8088",
                       delivery_base="http://localhost:8085", transfer_base="http://localhost:8090",
                       returning_base="http://localhost:8087", repairing_base="http://localhost:8091",
                       inspection_base="http://localhost:8089", scraping_base="http://localhost:8092",
                       token="...")
"""
from .core import (DEFAULT_ACCOUNT_BASE, DEFAULT_AUDIT_BASE, DEFAULT_DATA_BASE,
                   DEFAULT_DELIVERY_BASE, DEFAULT_INSPECTION_BASE,
                   DEFAULT_PRODUCTION_BASE, DEFAULT_PURCHASE_BASE,
                   DEFAULT_RECEIVING_BASE, DEFAULT_REPAIRING_BASE,
                   DEFAULT_RETURNING_BASE, DEFAULT_SALES_BASE, DEFAULT_SCRAPING_BASE,
                   DEFAULT_STOCK_BASE, DEFAULT_TRANSFER_BASE, FactorClient, FactorError)
from .biz_query import BizQueryApi
from .data_api import (CategoryQuery, ColorQuery, CustomerQuery, DataQueryApi,
                       MaterialQuery, ProductCatalogQuery, SupplierQuery,
                       UnitQuery, WarehouseQuery)
from .delivery_api import DELIVERY_STATUS, DeliveryApi
from .inspection_api import INSPECTION_ORDER_TYPE, INSPECTION_STATUS, InspectionApi
from .inventory_api import InventoryApi
from .production_api import PRODUCTION_ORDER_TYPE, PRODUCTION_STATUS, ProductionApi
from .purchase_api import ORDER_STATUS, PurchaseApi
from .receiving_api import RECEIPT_STATUS, ReceivingApi, receipt_status_name
from .repairing_api import REPAIR_STATUS, RepairingApi
from .returning_api import RETURN_STATUS, RETURN_TYPE, ReturningApi
from .sales_api import SALES_STATUS, SalesApi
from .scraping_api import SCRAP_STATUS, SCRAP_TYPE, ScrapingApi
from .transfer_api import TRANSFER_STATUS, TRANSFER_TYPE, TransferApi

__all__ = [
    "FactorApi", "factor", "FactorClient", "FactorError", "BizQueryApi",
    "DEFAULT_PURCHASE_BASE", "DEFAULT_DATA_BASE", "DEFAULT_ACCOUNT_BASE", "DEFAULT_AUDIT_BASE",
    "DEFAULT_RECEIVING_BASE", "DEFAULT_STOCK_BASE",
    "DEFAULT_SALES_BASE", "DEFAULT_PRODUCTION_BASE", "DEFAULT_DELIVERY_BASE",
    "DEFAULT_TRANSFER_BASE", "DEFAULT_RETURNING_BASE", "DEFAULT_REPAIRING_BASE",
    "DEFAULT_INSPECTION_BASE", "DEFAULT_SCRAPING_BASE",
    "ORDER_STATUS", "RECEIPT_STATUS", "SALES_STATUS", "PRODUCTION_STATUS",
    "PRODUCTION_ORDER_TYPE", "DELIVERY_STATUS", "TRANSFER_STATUS", "TRANSFER_TYPE",
    "RETURN_STATUS", "RETURN_TYPE", "REPAIR_STATUS", "INSPECTION_STATUS",
    "INSPECTION_ORDER_TYPE", "SCRAP_STATUS", "SCRAP_TYPE",
    "PurchaseApi", "ReceivingApi", "InventoryApi", "SalesApi", "ProductionApi",
    "DeliveryApi", "TransferApi", "ReturningApi", "RepairingApi", "InspectionApi",
    "ScrapingApi", "receipt_status_name",
    "DataQueryApi", "CategoryQuery", "ColorQuery", "CustomerQuery",
    "MaterialQuery", "ProductCatalogQuery", "SupplierQuery", "UnitQuery", "WarehouseQuery",
]


class FactorApi:
    """Factor 业务流门面(主数据 / 采购 / 收货 / 库存 / 销售 / 生产 / 发货 / 调拨 / 退货 / 维修 / 盘点 / 报废)。

    data=主数据查询;其余为业务单据查询与追踪。本技能不含任何写操作。
    """

    def __init__(self, purchase_base=None, data_base=None, account_base=None,
                 audit_base=None, receiving_base=None, stock_base=None,
                 sales_base=None, production_base=None, delivery_base=None,
                 transfer_base=None, returning_base=None, repairing_base=None,
                 inspection_base=None, scraping_base=None,
                 token=None, timeout=None):
        self.client = FactorClient(
            purchase_base=purchase_base, data_base=data_base,
            account_base=account_base, audit_base=audit_base,
            receiving_base=receiving_base, stock_base=stock_base,
            sales_base=sales_base, production_base=production_base,
            delivery_base=delivery_base, transfer_base=transfer_base,
            returning_base=returning_base, repairing_base=repairing_base,
            inspection_base=inspection_base, scraping_base=scraping_base,
            token=token, timeout=timeout,
        )
        self.data = DataQueryApi(self.client)
        self.receiving = ReceivingApi(self.client)
        self.inventory = InventoryApi(self.client)
        self.delivery = DeliveryApi(self.client)
        self.purchase = PurchaseApi(self.client,
                                    receiving=self.receiving,
                                    inventory=self.inventory)
        self.sales = SalesApi(self.client,
                              delivery=self.delivery,
                              inventory=self.inventory)
        self.production = ProductionApi(self.client)
        self.transfer = TransferApi(self.client)
        self.returning = ReturningApi(self.client)
        self.repairing = RepairingApi(self.client)
        self.inspection = InspectionApi(self.client)
        self.scraping = ScrapingApi(self.client)


# 默认实例:直连本机各服务端口,令牌由 cli.py 以 --token 注入
factor = FactorApi()
