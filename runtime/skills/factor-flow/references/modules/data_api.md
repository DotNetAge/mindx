# data_api.py — 主数据查询使用说明(data-service)

本模块集成 data-service 全部主数据对象的查询方法:分类、颜色、物料、成品目录、客户、供应商、计量单位、仓库(列表/详情/批量/BOM 反查/主仓)。

命令命名与「数据管理」技能同名命令一致(`supplier-list`/`supplier-get`/`material-find`/`material-bom`/`catalog-find`/`warehouse-primary` 等),便于 Agent 在技能间复用。

```
python ./scripts/api_client/cli.py --token <访问令牌> <实体>-<命令> [参数]
```

## 通用命令

每个实体都支持以下两条统一命令:

### <实体>-list [--page N] [--size N] [过滤字段=值 ...]

实体列表,返回 `{data, total, page, size}`。过滤字段按后端 `ListParams` **实际生效字段**白名单拦截(传其他字段报引导错误)。

```
python ./scripts/api_client/cli.py --token <访问令牌> supplier-list name=华东 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> warehouse-list warehouse_type=1 --size 20
```

### <实体>-get <ID>

按 ID 查询详情。

```
python ./scripts/api_client/cli.py --token <访问令牌> material-get m-1
```

## 实体与过滤字段一览

| 实体 | 命令前缀 | 路由组 | 支持过滤字段 |
|---|---|---|---|
| 分类 | `category-` | `/categories` | `name` |
| 颜色 | `color-` | `/colors` | `name` |
| 物料 | `material-` | `/materials` | `code`、`name`、`spec`、`color`、`unit`、`supplier_name` |
| 成品目录 | `catalog-` | `/product-catalogs` | `code`、`name`、`spec`、`color`、`category`、`unit` |
| 客户 | `customer-` | `/customers` | `code`、`name`、`contact_person`、`contact_phone` |
| 供应商 | `supplier-` | `/suppliers` | `code`、`name`、`contact_person`、`contact_phone` |
| 计量单位 | `unit-` | `/units` | `name` |
| 仓库 | `warehouse-` | `/warehouses` | `code`、`name`、`address`、`warehouse_type` |

> 注意:成品目录**不支持** `supplier_name` 过滤(Go 端已注释停用);物料**支持** `supplier_name`。

## 专用命令

### material-find <ID1,ID2,...>

物料按 ID 批量查询(逗号分隔),对应 `/materials/find-for`。

```
python ./scripts/api_client/cli.py --token <访问令牌> material-find m-1,m-2,m-3
```

### material-bom <成品ID>

按成品目录 ID **反查其 BOM 物料**,对应 `/materials/bom/:id`。命中取第一个匹配 BOM,无结果返回 404。

```
python ./scripts/api_client/cli.py --token <访问令牌> material-bom pc-1
```

### catalog-find <ID1,ID2,...>

成品目录按 ID 批量查询(逗号分隔),对应 `/product-catalogs/find-for`。

```
python ./scripts/api_client/cli.py --token <访问令牌> catalog-find pc-1,pc-2
```

### warehouse-primary

查询主仓,对应 `/warehouses/primary`(可能返回空数组)。

```
python ./scripts/api_client/cli.py --token <访问令牌> warehouse-primary
```

## 关键约定

- **查询用途**:Agent 查单据前先经本模块拿主数据 ID(供应商/仓库/物料/成品),不要凭空编造;追踪归因时用 `material-bom` 展开成品 BOM、`warehouse-primary` 定位主仓。
- **过滤白名单**:列表只接受后端实际生效字段,传其他字段报引导错误(原因/下一步可参见 core.md)。
- **维护边界**:创建/更新/删除/报价/BOM 追加等写操作由「数据管理」技能完成,本模块不提供。
