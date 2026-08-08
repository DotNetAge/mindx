# cli.py — 命令行入口使用说明

本文件是本技能**唯一的操作入口**。Agent 的一切业务操作都通过 `python <脚本路径> <命令>` 执行,不需要(也无法)直接 import 包。

- **脚本位置(相对项目根)**:`./scripts/api_client/cli.py`
- **运行方式**(命令均在技能目录下执行):

```
python ./scripts/api_client/cli.py --token <访问令牌> <命令> [参数]
```

命令带**模块前缀**区分业务场景(完整分流见 SKILL.md 业务全景表):**采购(`po-`)、收货(`rn-`)、库存(`inv-`)、销售(`so-`)、生产(`prd-`)、发货(`dn-`)、调拨(`tr-`)、退货(`ro-`)、维修(`rp-`)、盘点(`ins-`)、报废(`sc-`)、主数据查询(`<实体>-`)**。

## 令牌

所有命令都需要 `--token <访问令牌>`(获取方式见 usage.md)。

## 命令总览

### 采购模块(po-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `po-list [--page N] [--size N] [过滤字段=值 ...]` | 采购单列表(含明细) | purchase_api.md |
| 查询 | `po-get <采购单ID>` | 采购单详情 | purchase_api.md |
| 查询 | `po-counting --status N` | 按状态计数 | purchase_api.md |
| 查询 | `po-find-for <ID1,ID2,...>` | 按 ID 批量查询 | purchase_api.md |
| 查询 | `po-export <采购单ID> [--out 路径]` | 导出单个采购单 Excel | purchase_api.md |
| **追踪** | **`po-track <采购单ID或CG单号>`** | **按单号追踪到货(收货单)+ 库存核对 + 剩余应到货量** | purchase_api.md + guides/purchasing.md |

> 采购单状态:`1` 待审核、`2` 已同意、`3` 部分收货、`4` 已收货、`5` 已完成、`6` 已拒绝、`7` 已取消。下单/审核/取消/完成等写操作由 WebUI 完成。

### 收货模块(rn-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `rn-list [--page N] [--size N] [过滤字段=值 ...]` | 收货单列表 | receiving_api.md |
| 查询 | `rn-get <收货单ID>` | 收货单详情(含明细) | receiving_api.md |
| 查询 | `rn-find-for <ID1,ID2,...>` | 按 ID 批量查询 | receiving_api.md |

> 收货单状态:`1` 草稿/准备中、`2` 待收货、`3` 已完成、`4` 已取消。追踪到货优先用 `po-track`。

### 库存模块(inv-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `inv-list [--page N] [--size N] [过滤字段=值 ...]` | 物理库存列表(**不支持 item_id**) | inventory_api.md |
| 查询 | `inv-get <库存记录ID>` | 单条库存记录详情 | inventory_api.md |
| 查询 | `inv-warehouse-summary warehouse_id=ID item_id=ID` | **仓库库存摘要**(核对某仓库某物料库存) | inventory_api.md |
| 查询 | `inv-sku-summary [item_id=ID]` | SKU 摘要(跨仓库按物料) | inventory_api.md |
| 查询 | `inv-supplier-summary [supplier_id=ID]` | 供应商库存摘要 | inventory_api.md |

> 核对"采购数量 vs 库存"优先用 `po-track`;`inv-*` 用于手动查库存明细。

### 销售模块(so-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `so-list [--page N] [--size N] [过滤字段=值 ...]` | 销售订单列表 | sales_api.md |
| 查询 | `so-get <销售订单ID>` | 销售订单详情(含明细) | sales_api.md |
| **追踪** | **`so-track <销售订单ID或XS单号>`** | **追踪发货:发货单 + 剩余应发量 + 库存支撑核对** | sales_api.md + guides/tracking.md |

> 销售订单状态:`1` 待发货、`2` 已部分发货、`3` 已发货、`4` 已完成、`5` 已取消。

### 生产模块(prd-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `prd-list [--page N] [--size N] [过滤字段=值 ...]` | 生产工单列表 | production_api.md |
| 查询 | `prd-get <生产工单ID>` | 生产工单详情(含明细) | production_api.md |
| 查询 | `prd-find-for <ID1,ID2,...>` | 按 ID 批量查询 | production_api.md |

> 生产工单状态:`1` 待处理、`2` 已接单、`3` 生产中、`4` 已完成、`5` 已挂起、`6` 已拒绝、`7` 已取消。

### 发货模块(dn-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `dn-list [--page N] [--size N] [过滤字段=值 ...]` | 发货单(出库单)列表 | delivery_api.md |
| 查询 | `dn-get <发货单ID>` | 发货单详情(含明细) | delivery_api.md |
| 查询 | `dn-find-for <ID1,ID2,...>` | 按 ID 批量查询 | delivery_api.md |

> 发货单状态:`1` 待审核、`2` 已同意、`3` 已完成、`4` 部分发货、`5` 已拒绝、`6` 已取消。

### 调拨模块(tr-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `tr-list [--page N] [--size N] [过滤字段=值 ...]` | 调拨单列表 | transfer_api.md |
| 查询 | `tr-get <调拨单ID>` | 调拨单详情(含明细) | transfer_api.md |

> 调拨单状态:`1` 待审核、`2` 已同意、`3` 调出完成、`4` 调入开始、`5` 已完成、`6` 已拒绝、`7` 已取消。

### 退货模块(ro-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `ro-list [--page N] [--size N] [过滤字段=值 ...]` | 退货单列表 | return_api.md |
| 查询 | `ro-get <退货单ID>` | 退货单详情(含明细) | return_api.md |

> 退货单状态:`1` 待审核、`2` 已同意、`3` 已完成、`4` 已拒绝、`5` 已取消。

### 维修模块(rp-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `rp-list [--page N] [--size N] [过滤字段=值 ...]` | 维修单列表 | repair_api.md |
| 查询 | `rp-get <维修单ID>` | 维修单详情(含明细) | repair_api.md |

> 维修单状态:`1` 待审核、`2` 已同意、`3` 维修中、`4` 已完成、`5` 已拒绝、`6` 已取消。

### 盘点模块(ins-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `ins-list [--page N] [--size N] [过滤字段=值 ...]` | 盘点单列表 | inspection_api.md |
| 查询 | `ins-get <盘点单ID>` | 盘点单详情(含明细) | inspection_api.md |

> 盘点单状态:`1` 待审核、`2` 盘点中、`3` 已完成、`4` 已取消。**注意:盘点列表接口不带 status 时后端按 Status=0 过滤(结果恒空),查列表务必带 status。**

### 报废模块(sc-)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `sc-list [--page N] [--size N] [过滤字段=值 ...]` | 报废单列表 | scrap_api.md |
| 查询 | `sc-get <报废单ID>` | 报废单详情(含明细) | scrap_api.md |

> 报废单状态:`1` 待审核、`2` 已同意、`3` 处理中、`4` 已完成、`5` 已拒绝、`6` 已取消。

### 主数据查询(data-service 实体)

| 分组 | 命令 | 用途 | 详见 |
|---|---|---|---|
| 查询 | `<实体>-list [--page N] [--size N] [过滤字段=值 ...]` | 实体列表 | data_api.md |
| 查询 | `<实体>-get <ID>` | 实体详情 | data_api.md |
| 查询 | `material-find <ID1,ID2,...>` | 物料按 ID 批量查询 | data_api.md |
| 查询 | `material-bom <成品ID>` | 按成品 ID 反查 BOM 物料 | data_api.md |
| 查询 | `catalog-find <ID1,ID2,...>` | 成品目录按 ID 批量查询 | data_api.md |
| 查询 | `warehouse-primary` | 查询主仓 | data_api.md |

> `<实体>` ∈ `category`(分类)、`color`(颜色)、`customer`(客户)、`material`(物料)、`catalog`(成品目录)、`supplier`(供应商)、`unit`(计量单位)、`warehouse`(仓库)。命名与「数据管理」技能同名命令一致,便于 Agent 复用。主数据维护(创建/更新/删除)由「数据管理」技能完成。

## 通用规则

- **参数写法**:所有命令的过滤条件用 `字段=值` 传参(如 `status=1`、`supplier_name=华东`、`order_no=CG2608061234`);列表/批量参数用逗号分隔(如 `material-find m-1,m-2`)。
- **输出**:成功输出 JSON;失败输出「问题→原因→下一步」引导提示(见 core.md),按提示修正后重试。
- **先查后用**:供应商/仓库/物料/成品 ID 必须先查询获得,不要凭空编造;主数据查询与业务单据查询交叉使用。
- **过滤白名单**:列表接口只接受后端实际生效的过滤字段,不在白名单内的字段会被拦截并提示可选字段。

## 快速示例

```
# 1. 先在「授权与账号」技能目录登录,取 data.access_token(在 authorization-account 技能目录执行)
python ./scripts/cli.py login alice secret123

# 2. 主数据查询(data-service 实体,命名与「数据管理」技能一致)
python ./scripts/api_client/cli.py --token <访问令牌> supplier-list name=华东 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> warehouse-list warehouse_type=1 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> material-find m-1,m-2
python ./scripts/api_client/cli.py --token <访问令牌> material-bom pc-1        # 按成品反查 BOM
python ./scripts/api_client/cli.py --token <访问令牌> warehouse-primary

# 3. 查采购单
python ./scripts/api_client/cli.py --token <访问令牌> po-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> po-list status=1 --size 50

# 4. 追踪到货情况(按单号或 ID):收货单 + 库存核对 + 剩余应到货量
python ./scripts/api_client/cli.py --token <访问令牌> po-track CG2608061234
python ./scripts/api_client/cli.py --token <访问令牌> po-track po-1

# 5. 手动查收货单 / 核对某仓库某物料库存
python ./scripts/api_client/cli.py --token <访问令牌> rn-list document_type=PurchaseOrder status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> inv-warehouse-summary warehouse_id=wh-1 item_id=m-9

# 6. 跨模块查询业务单据(查询是 Agent 的主场)
python ./scripts/api_client/cli.py --token <访问令牌> so-list status=3 --size 20          # 销售:已发货
python ./scripts/api_client/cli.py --token <访问令牌> so-track XS2608061234               # 销售追踪发货
python ./scripts/api_client/cli.py --token <访问令牌> prd-list status=3 --size 20         # 生产:生产中
python ./scripts/api_client/cli.py --token <访问令牌> dn-list status=3 --size 20          # 发货:已完成
python ./scripts/api_client/cli.py --token <访问令牌> tr-list status=5 --size 20          # 调拨:已完成
python ./scripts/api_client/cli.py --token <访问令牌> ro-list status=2 --size 20          # 退货:已同意
python ./scripts/api_client/cli.py --token <访问令牌> rp-list status=3 --size 20          # 维修:维修中
python ./scripts/api_client/cli.py --token <访问令牌> ins-list status=2 --size 20         # 盘点:盘点中
python ./scripts/api_client/cli.py --token <访问令牌> sc-list status=4 --size 20          # 报废:已完成
```
