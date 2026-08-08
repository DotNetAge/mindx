"""data-service(主数据)查询模块:分类/颜色/物料/成品目录/客户/供应商/计量单位/仓库 的查询方法。

本模块只提供查询(list/get/find-for/bom/primary),**不含任何写操作**;
主数据维护(创建/更新/删除/报价/BOM 追加等)由「数据管理」技能完成。
列表过滤字段按后端 ListParams 实际生效字段白名单拦截(与各服务 queries.go 一致)。
"""
from __future__ import annotations

from typing import Any, Dict, List, Optional

from .core import FactorClient, FactorError


class DataQuery:
    """主数据实体查询基类:统一封装列表(list)与详情(get),子类声明过滤白名单。"""

    def __init__(self, client: FactorClient, group: str, entity: str,
                 search_fields: Optional[List[str]] = None):
        self.c = client
        self.group = group          # 路由组,如 materials
        self.entity = entity        # 中文名,如 物料
        self.search_fields = list(search_fields or [])

    def list(self, page: int = 1, size: int = 10, **filters) -> Dict[str, Any]:
        """列表查询:GET /{group}/,支持 page/size 与白名单过滤字段。"""
        func = f"{self.entity}.list"
        params: Dict[str, Any] = {"page": page, "size": size}
        for k, v in filters.items():
            if k not in self.search_fields:
                raise FactorError(
                    func=func, message=f"不支持的过滤字段:{k}",
                    reason=f"{self.entity} 列表只支持过滤字段:" + "、".join(self.search_fields),
                    hint="请从支持的过滤字段中选择,或去掉该参数",
                )
            if v is None or v == "":
                continue
            params[k] = v
        body = self.c.request(func, "GET", self.c.data_base, f"/{self.group}/", params=params)
        data = body.get("data")
        total = body.get("total")
        if isinstance(data, dict):
            data = data.get("list", data.get("data", []))
        if total is None and isinstance(data, list):
            total = len(data)
        return {"data": data or [], "total": total, "page": page, "size": size}

    def get(self, id: str) -> Dict[str, Any]:
        """按 ID 查询详情:GET /{group}/:id。"""
        func = f"{self.entity}.get"
        if not id:
            raise FactorError(func=func, message="缺少 ID 参数",
                              reason="查询详情必须提供资源 ID",
                              hint=f"请先调用 {self.entity}.list() 获取 ID 后再调用")
        body = self.c.request(func, "GET", self.c.data_base, f"/{self.group}/{id}")
        return body.get("data") or body

    def find_by_ids(self, ids: List[str]) -> List[Dict[str, Any]]:
        """按 ID 批量查询:GET /{group}/find-for?ids=a,b,c(仅后端提供 find-for 的实体)。"""
        func = f"{self.entity}.find_by_ids"
        if not ids:
            raise FactorError(func=func, message="ids 列表为空",
                              reason="批量查询至少需要一个 ID",
                              hint="请提供资源 ID 列表")
        body = self.c.request(func, "GET", self.c.data_base, f"/{self.group}/find-for",
                              params={"ids": ",".join(ids)})
        return body.get("data") or []


class CategoryQuery(DataQuery):
    """分类:name 过滤。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "categories", "分类", search_fields=["name"])


class ColorQuery(DataQuery):
    """颜色:name 过滤。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "colors", "颜色", search_fields=["name"])


class MaterialQuery(DataQuery):
    """物料:code/name/spec/color/unit/supplier_name 过滤;支持按 ID 批量查询与 BOM 反查。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "materials", "物料",
                         search_fields=["code", "name", "spec", "color", "unit", "supplier_name"])

    def bom_items(self, catalog_id: str) -> List[Dict[str, Any]]:
        """按成品 ID 反查其 BOM 物料:GET /materials/bom/:id(命中取第一个,无结果 404)。"""
        func = "物料.bom_items"
        if not catalog_id:
            raise FactorError(func=func, message="缺少成品 ID",
                              reason="BOM 反查需要提供成品目录 ID",
                              hint="请先查询成品目录获取 ID")
        body = self.c.request(func, "GET", self.c.data_base, f"/materials/bom/{catalog_id}")
        data = body.get("data") or body
        return data if isinstance(data, list) else [data]


class ProductCatalogQuery(DataQuery):
    """成品目录:code/name/spec/color/category/unit 过滤;支持按 ID 批量查询。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "product-catalogs", "成品目录",
                         search_fields=["code", "name", "spec", "color", "category", "unit"])


class CustomerQuery(DataQuery):
    """客户:code/name/contact_person/contact_phone 过滤。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "customers", "客户",
                         search_fields=["code", "name", "contact_person", "contact_phone"])


class SupplierQuery(DataQuery):
    """供应商:code/name/contact_person/contact_phone 过滤。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "suppliers", "供应商",
                         search_fields=["code", "name", "contact_person", "contact_phone"])


class UnitQuery(DataQuery):
    """计量单位:name 过滤。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "units", "计量单位", search_fields=["name"])


class WarehouseQuery(DataQuery):
    """仓库:code/name/address/warehouse_type 过滤;支持主仓查询。"""

    def __init__(self, client: FactorClient):
        super().__init__(client, "warehouses", "仓库",
                         search_fields=["code", "name", "address", "warehouse_type"])

    def primary(self) -> List[Dict[str, Any]]:
        """查询主仓:GET /warehouses/primary(可能为空数组)。"""
        body = self.c.request("仓库.primary", "GET", self.c.data_base, "/warehouses/primary")
        return body.get("data") or []


class DataQueryApi:
    """data-service 查询门面,持有 8 个主数据实体查询实例(与「数据管理」技能同名命令一致)。"""

    def __init__(self, client: FactorClient):
        self.client = client
        self.category = CategoryQuery(client)
        self.color = ColorQuery(client)
        self.customer = CustomerQuery(client)
        self.material = MaterialQuery(client)
        self.product_catalog = ProductCatalogQuery(client)
        self.supplier = SupplierQuery(client)
        self.unit = UnitQuery(client)
        self.warehouse = WarehouseQuery(client)
