# inventory_api.py — 库存模块使用说明

库存模块查询命令,带 `inv-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> inv-<命令> [参数]
```

## 库存是什么

库存(stock-service)是 CQRS **读模型**:收货单「已完成」事件驱动库存增加(`PushItem +Quantity`),出库为负数。每次入库是一条独立库存记录。

每条库存记录含 `batch_id`(批次号,由入库当日日期生成,如 2026-08-06 的批次号为 `20260806`),库存按"产品ID + 供应商ID + 仓库ID + 原单号 + 批次ID"唯一标识。**但批次号只是记录维度,不是查询过滤维度**:`inv-list` 的 `batch_id` 过滤在后端未启用(查询条件被注释),`warehouse/sku/supplier-summary` 汇总视图也不含批次维度。

## 四个查询视图(选哪个很重要)

| 命令 | 视图 | 用途 | item_id 匹配 |
|---|---|---|---|
| `inv-list` | 物理库存列表 | 按仓库/供应商/编码/名称看明细记录 | **不支持** |
| `inv-warehouse-summary` | 仓库库存摘要 | **核对某仓库某物料当前库存**(与 `po-track` 内部逻辑一致) | 精确(EQ) |
| `inv-sku-summary` | SKU 摘要 | 跨仓库按物料汇总 | 模糊(LIKE) |
| `inv-supplier-summary` | 供应商摘要 | 按供应商/仓库/物料汇总 | 模糊(LIKE) |

> 三个 summary 视图都强制 `Quantity > 0`(零库存不显示)。warehouse/sku 按库存量降序, supplier 按供应商名+物料名降序。

## 查询命令

### inv-list [--page N] [--size N] [过滤字段=值 ...]

物理库存列表,返回 `{data, total, page, size}`。

支持过滤字段:`warehouse_id`、`warehouse_name`、`supplier_id`、`supplier_name`、`code`、`name`。**注意:此接口不支持 item_id 过滤,也不支持 batch_id 过滤(后端未启用)**;核对某物料库存请用 `inv-warehouse-summary`。

```
python ./scripts/api_client/cli.py --token <访问令牌> inv-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> inv-list warehouse_id=wh-1 supplier_id=sup-1
python ./scripts/api_client/cli.py --token <访问令牌> inv-list code=ML-001
```

记录字段:`warehouse_id/name`、`batch_id`(批次号,仅展示不可过滤)、`item_id`、`item_type`、`code`、`name`、`category`、`spec`、`color`、`unit`、`price`、`quantity`、`supplier_id/name`、`remarks`。

### inv-get <库存记录ID>

按 ID 查询单条库存记录详情。

### inv-warehouse-summary [--page N] [--size N] [过滤字段=值 ...]

**仓库库存摘要**(核对收货仓库库存用它)。支持 `warehouse_id` + `item_id` **精确**过滤,强制 `Quantity > 0`,按库存量降序。

```
python ./scripts/api_client/cli.py --token <访问令牌> inv-warehouse-summary warehouse_id=wh-1 item_id=m-9
python ./scripts/api_client/cli.py --token <访问令牌> inv-warehouse-summary warehouse_name=原材料仓 --size 50
```

### inv-sku-summary [--page N] [--size N] [过滤字段=值 ...]

SKU 库存摘要(跨仓库按物料汇总)。支持 `item_id`、`code`、`name`(item_id 为**模糊**匹配)。

```
python ./scripts/api_client/cli.py --token <访问令牌> inv-sku-summary item_id=m-9
```

### inv-supplier-summary [--page N] [--size N] [过滤字段=值 ...]

供应商库存摘要。支持 `warehouse_id`、`warehouse_name`、`supplier_id`、`supplier_name`、`item_id`、`code`、`name`。

```
python ./scripts/api_client/cli.py --token <访问令牌> inv-supplier-summary supplier_id=sup-1 warehouse_id=wh-1
```

## 关键约定

- **核对"采购数量 vs 库存"请用 `po-track`**:它自动按收货仓库 + 物料查询 warehouse-summary 并给出结论;`inv-*` 用于手动查库存明细。
- **库存是存量**:当前库存为历史累计(可能因领用/出库减少),不等于"本次入库增量";系统无库存流水接口。
