# inspection_api.py — 盘点模块使用说明

盘点模块查询命令,带 `ins-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> ins-<命令> [参数]
```

## 盘点单是什么

盘点单(InventoryCheckOrder)用于核对仓库实物与账面库存,由 inspection-service 提供查询:

- 状态流转:**待审核 → 盘点中 → 已完成**;可取消;
- 盘点类型:`0` 全面盘点、`1` 部分盘点、`2` 循环盘点;
- 盘点仓库 `warehouse_id/name`,盘点方式 `method`、周期 `cycle`;
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`(账面库存数量)、`actual_qty`(实盘数量)、`diff_qty`(差异数量)、`diff_reason`(差异原因)、`process_method`(处理方式)、`diff_amount`(差异金额)。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 盘点单已创建,待审核 |
| 2 | 盘点中 | 盘点进行中 |
| 3 | 已完成 | 盘点完成 |
| 4 | 已取消 | 已取消 |

## 盘点类型枚举

| 值 | 类型 |
|---|---|
| 0 | 全面盘点 |
| 1 | 部分盘点 |
| 2 | 循环盘点 |

## 查询命令

### ins-list [--page N] [--size N] [过滤字段=值 ...]

盘点单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`warehouse_name`(仓库名,模糊)、`warehouse_id`(仓库 ID,精确)、`status`(状态,精确)。

> **重要:列表接口务必带 `status`**。后端对 `status` 的过滤条件没有"未传则跳过"的守卫(其他模块均有),不传时后端按 `Status=0` 过滤,结果恒为空列表。

```
python ./scripts/api_client/cli.py --token <访问令牌> ins-list status=1 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ins-list status=2 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ins-list warehouse_id=wh-1 status=3 --size 20
```

### ins-get <盘点单ID>

按 ID 查询盘点单详情(含 items 明细,差异数量在 `diff_qty`)。

```
python ./scripts/api_client/cli.py --token <访问令牌> ins-get ins-1
```

## 关键约定

- 盘点发起/提交等写操作由 WebUI 完成。
- 盘点用于**对账**:拿 `diff_qty`(实盘−账面)与库存查询(`inv-*`)交叉核对。
- 过滤字段受限,客户端已做白名单拦截(如按盘点类型 `order_type` 过滤在服务端未使用,故不支持)。
