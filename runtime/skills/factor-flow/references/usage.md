# factor-flow:使用说明(总览)

本技能聚合 Factor 平台**全部业务单据流**。一切操作都通过 **python 指令**执行,入口脚本(命令均在技能目录下执行,路径用相对引用):

```
python ./scripts/api_client/cli.py --token <访问令牌> <命令> [参数]
```

命令带**模块前缀**区分业务(如 `po-*` 采购、`rn-*` 收货、`inv-*` 库存、`<实体>-*` 主数据查询……),完整分流见 `SKILL.md` 业务全景表。

裸 API 仅作存档(`api/`),不用于调用。

## 令牌怎么来

本技能**不负责登录**。访问令牌通过「授权与账号」技能获取:

1. 在「授权与账号」技能目录执行登录,输出完整 JSON(不落盘):
   `python ./scripts/cli.py login 用户名 密码`
2. 从输出中取 `data.access_token`,暂存于对话上下文。
3. 本技能所有命令都以 `--token <访问令牌>` 传入。
4. 令牌有效期 2 小时,过期(HTTP 401)后重新登录。

## 主数据怎么来

- **查询**:本技能直接集成 data-service 全部主数据对象的查询方法(`category`/`color`/`customer`/`material`/`catalog`/`supplier`/`unit`/`warehouse` 的 list/get,物料批量与 BOM 反查、仓库主仓),命名与「数据管理」技能一致。
- **维护**:创建/更新/删除/报价/BOM 等写操作由「数据管理」技能完成,本技能不提供。

## 快速开始(采购场景)

```
# 1. 先经「授权与账号」技能登录,取 data.access_token(在 authorization-account 技能目录执行)
python ./scripts/cli.py login 用户名 密码

# 2. 主数据查询(实体 ID 先经查询获得)
python ./scripts/api_client/cli.py --token <访问令牌> supplier-list name=华东 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> material-bom <成品ID>

# 3. 追踪采购单到货情况(按单号或 ID):收货单 + 库存核对 + 剩余应到货量
python ./scripts/api_client/cli.py --token <访问令牌> po-track CG2608061234

# 4. 查询各业务单据(查询是 Agent 的主场)
python ./scripts/api_client/cli.py --token <访问令牌> so-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> prd-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> dn-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> tr-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ro-list status=2 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> rp-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ins-list status=2 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> sc-list status=4 --size 20

# 5. 追踪销售订单发货情况(销售单号或 ID):发货单 + 剩余应发量 + 库存支撑核对
python ./scripts/api_client/cli.py --token <访问令牌> so-track XS2608061234
```

## 命令分组速查(全部业务模块)

### 采购(po-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `po-list` / `po-get` / `po-counting` / `po-find-for` / `po-export` | 采购单查询与导出 | modules/purchase_api.md |
| **追踪** | **`po-track <采购单ID或CG单号>`** | **按单号追踪到货(收货单)+ 库存核对 + 剩余应到货量** | modules/purchase_api.md + guides/purchasing.md |

> 采购单状态:`1` 待审核、`2` 已同意、`3` 部分收货、`4` 已收货、`5` 已完成、`6` 已拒绝、`7` 已取消。下单/审核/取消/完成等写操作由 WebUI 完成。

### 收货(rn-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `rn-list` / `rn-get` / `rn-find-for` | 收货单查询(追踪到货) | modules/receiving_api.md + guides/receiving.md |

### 库存(inv-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `inv-list` / `inv-get` | 物理库存明细 | modules/inventory_api.md + guides/inventory.md |
| 查询 | `inv-warehouse-summary` / `inv-sku-summary` / `inv-supplier-summary` | 库存摘要(核对库存) | modules/inventory_api.md + guides/inventory.md |

### 销售(so-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `so-list` / `so-get` | 销售订单查询(状态:1待发货/2部分发货/3已发货/4已完成/5已取消) | modules/sales_api.md + guides/sales.md |
| **追踪** | **`so-track <销售订单ID或XS单号>`** | **追踪发货:发货单 + 每物料剩余应发量 + 发货仓库库存支撑核对** | modules/sales_api.md + guides/sales.md + guides/tracking.md |

### 生产(prd-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `prd-list` / `prd-get` / `prd-find-for` | 生产工单查询(状态:1待处理/2已接单/3生产中/4已完成/5已挂起/6已拒绝/7已取消) | modules/production_api.md + guides/production.md |

### 发货(dn-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `dn-list` / `dn-get` / `dn-find-for` | 发货单查询(状态:1待审核/2已同意/3已完成/4部分发货/5已拒绝/6已取消) | modules/delivery_api.md + guides/delivery.md |

### 调拨(tr-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `tr-list` / `tr-get` | 调拨单查询(状态:1待审核/2已同意/3调出完成/4调入开始/5已完成/6已拒绝/7已取消) | modules/transfer_api.md + guides/transfer.md |

### 退货(ro-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `ro-list` / `ro-get` | 退货单查询(状态:1待审核/2已同意/3已完成/4已拒绝/5已取消) | modules/return_api.md + guides/return.md |

### 维修(rp-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `rp-list` / `rp-get` | 维修单查询(状态:1待审核/2已同意/3维修中/4已完成/5已拒绝/6已取消) | modules/repair_api.md + guides/repair.md |

### 盘点(ins-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `ins-list` / `ins-get` | 盘点单查询(状态:1待审核/2盘点中/3已完成/4已取消;列表务必带 status) | modules/inspection_api.md + guides/inspection.md |

### 报废(sc-)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `sc-list` / `sc-get` | 报废单查询(状态:1待审核/2已同意/3处理中/4已完成/5已拒绝/6已取消) | modules/scrap_api.md + guides/scrap.md |

### 主数据查询(data-service 实体)

| 分组 | 命令 | 用途 | 详细参考 |
|---|---|---|---|
| 查询 | `<实体>-list` / `<实体>-get` | 分类/颜色/客户/物料/成品目录/供应商/计量单位/仓库 列表与详情 | modules/data_api.md |
| 查询 | `material-find` / `catalog-find` | 物料/成品目录按 ID 批量查询 | modules/data_api.md |
| 查询 | `material-bom <成品ID>` | 按成品 ID 反查 BOM 物料 | modules/data_api.md |
| 查询 | `warehouse-primary` | 查询主仓 | modules/data_api.md |

> `<实体>` ∈ `category`/`color`/`customer`/`material`/`catalog`/`supplier`/`unit`/`warehouse`。命名与「数据管理」技能一致,便于 Agent 复用;主数据维护由「数据管理」技能完成。

> 本技能只提供查询与追踪,不含任何写操作;所有写操作均由 WebUI 完成,主数据维护由「数据管理」技能完成。

## 阅读顺序(找不到资料时)

1. **`SKILL.md`**:业务全景与分流(定位业务场景)。
2. **本文件**:令牌、命令分组、阅读顺序。
3. **`modules/cli.md`**:全部命令总参考(入口)。
4. **`modules/<模块>.md`**:具体命令的参数、过滤字段、示例(业务模块 + `data_api.md` 主数据查询)。
5. **`modules/core.md`**:错误提示与校验规则的含义。
6. **`guides/tracking.md`**:追踪总纲(所有追踪方案的方法论,以库存/残次仓对齐为终点)。
7. **`guides/<场景>.md`**:业务规则(怎么做,如采购流程、状态机)。
8. **`api/`**:裸 API 存档(仅需底层信息时查阅)。

## 关键约定

- **先查后用**:供应商/仓库/物料/成品 ID 必须先通过查询获得。
- **过滤白名单**:列表接口只接受后端实际生效的过滤字段,不在白名单内的字段会被拦截并提示可选字段。
- **追踪终点**:一切追踪方案都以库存/残次仓数量对齐为终点(见 guides/tracking.md)。
- **错误处理**:所有失败输出「问题→原因→下一步」,直接按提示修正重试。
