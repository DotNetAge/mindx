# transfer_api.py — 调拨模块使用说明

调拨模块查询命令,带 `tr-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> tr-<命令> [参数]
```

## 调拨单是什么

调拨单(TransferOrder)在仓库之间转移物料,由 transfer-service 提供查询:

- 状态流转:**待审核 → 已同意 → 调出完成 → 调入开始 → 已完成**;可拒绝/取消;
- 调出仓库 `source_warehouse_id/name`,调入仓库 `target_warehouse_id/name`;
- 调拨类型:`0` 正常调拨、`1` 紧急调拨、`2` 库存平衡、`3` 生产需求;
- 关联单据:`delivery_note_id`(调出仓出库单)、`receipt_note_id`(调入仓入库单)——调出出库完成 → 「调出完成」,调入入库创建/完成 → 「调入开始/已完成」;
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`、`supplier_id/name`。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 调拨单已提交,待审核 |
| 2 | 已同意 | 审核通过 |
| 3 | 调出完成 | 调出仓已出库 |
| 4 | 调入开始 | 调入仓已生成入库单 |
| 5 | 已完成 | 调入仓已入库,调拨完成 |
| 6 | 已拒绝 | 已拒绝 |
| 7 | 已取消 | 已取消 |

## 调拨类型枚举

| 值 | 类型 |
|---|---|
| 0 | 正常调拨 |
| 1 | 紧急调拨 |
| 2 | 库存平衡 |
| 3 | 生产需求 |

## 查询命令

### tr-list [--page N] [--size N] [过滤字段=值 ...]

调拨单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`source_warehouse_name`(调出仓名,模糊)、`source_warehouse_id`(调出仓 ID,精确)、`target_warehouse_name`(调入仓名,模糊)、`target_warehouse_id`(调入仓 ID,精确)、`status`(状态,精确)、`creator_name`(申请人,模糊)。

```
python ./scripts/api_client/cli.py --token <访问令牌> tr-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> tr-list status=5 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> tr-list source_warehouse_id=wh-1 target_warehouse_id=wh-2
```

### tr-get <调拨单ID>

按 ID 查询调拨单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> tr-get tr-1
```

## 关键约定

- 调拨发起/审核等写操作由 WebUI 完成。
- 调拨是「调出出库 → 调入入库」的链路:调出看 `delivery_note_id` 发货单,调入看 `receipt_note_id` 收货单,库存核对用 `inv-*`。
- 过滤字段受限,客户端已做白名单拦截(如按调拨类型 `type` 过滤在服务端未生效,故不支持)。
