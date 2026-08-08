# purchase_api.py — 采购模块使用说明

采购模块查询与追踪命令,带 `po-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-<命令> [参数]
```

## 查询命令

### po-list [--page N] [--size N] [过滤字段=值 ...]

采购单列表(含明细 items),返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`status`(状态,精确)、`supplier_name`(供应商,模糊)、`handler_name`(经手人,模糊)、`ordered_at`(下单日期)、`warehouse_id`(仓库)。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> po-list status=1 --size 50
python ./scripts/api_client/cli.py --token <访问令牌> po-list supplier_name=华东 --size 20
```

- `ordered_at` 支持 `YYYY-MM-DD` 或 `YYYY-MM-DD HH:mm:ss`,自动转 ISO 时间戳。
- 明细行字段:`item_id`、`code`、`name`、`category`、`spec`、`color`、`unit`、`price`、`quantity`、`received_qty`、`remarks`。

### po-get <采购单ID>

按 ID 查询详情(含 items、receipt_notes、审核/取消/完成信息)。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-get po-1
```

### po-counting --status N

按状态计数。状态:`1` 待审核 / `2` 已同意 / `3` 部分收货 / `4` 已收货 / `5` 已完成 / `6` 已拒绝 / `7` 已取消。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-counting --status 1
```

### po-find-for <ID1,ID2,...>

按 ID 批量查询(逗号分隔)。

### po-export <采购单ID> [--out 路径]

导出单个采购单 Excel。`--out` 指定保存路径;省略则只输出字节数(不落盘)。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-export po-1 --out ./采购单.xlsx
```

## 追踪命令(采购单 -> 收货 -> 库存)

### po-track <采购单ID或CG单号>

**按采购单号追踪到货情况**——下采购单后最重要的一步:是否已经到货(收货单)、收货仓库库存是否正确增加、采购数量与库存增加是否一致、部分收货后还剩多少物料待收货。输入支持采购单 ID 或以 `CG` 开头的采购单号(自动按单号定位)。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-track po-1
python ./scripts/api_client/cli.py --token <访问令牌> po-track CG2608061234
```

返回的追踪视图(JSON)包含四部分:

| 部分 | 内容 |
|---|---|
| `purchase_order` | 采购单基本信息:单号、供应商、收货仓库、状态(中文)、金额、需求日期、关联收货单 ID 列表 |
| `receipt_notes` | 每张关联收货单:单号、状态、收货仓库、收货日期、经手人、明细(各物料收货数量) |
| `items` | **每物料核对表**:采购量 `quantity`、已收货量 `received_qty`、剩余应到货量 `remaining_qty`、到货状态 `match`、各收货单收货明细 `receipts`、收货仓库当前库存 `stock` |
| `checks` | **检查点报告(核心,供 LLM 归因)**:每个检查点含 `check`(名称)/`ok`(程序判定)/`detail`(依据)/`guide`(导引词) |
| `summary` | 汇总:到齐/部分/未到货项数、`pass_count`/`fail_count` 与 `conclusion`(结论) |

**检查点列表**与各检查点的失败导引词见 `guides/tracking.md`(po-track 一节)。

> **说明**:`stock.stock_qty` 是**当前存量**(历史累计,可能因领用/出库减少),不是"本次入库增量";系统无库存流水接口,因此一致性核对以"已收量 = 采购量"与"库存覆盖已收量"为准。收货单查询失败时不会阻断,在 `receipt_notes_error` 中给出原因。

## 关键约定

- 状态枚举:`1` 待审核 / `2` 已同意 / `3` 部分收货 / `4` 已收货 / `5` 已完成 / `6` 已拒绝 / `7` 已取消。
- 追踪到货优先用 `po-track`;需查收货单明细时用 `rn-*`,核对库存用 `inv-*`。
- 列表接口只接受后端实际生效的过滤字段,不在白名单内的字段会被拦截并提示可选字段。
