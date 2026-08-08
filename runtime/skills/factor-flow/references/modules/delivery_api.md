# delivery_api.py — 发货模块使用说明

发货模块查询命令,带 `dn-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> dn-<命令> [参数]
```

## 发货单是什么

发货单(DeliveryNote)由销售订单发货或调拨调出等场景产生,由 delivery-service 提供查询:

- 状态流转:**待审核 → 已同意 → 已完成/部分发货**;可拒绝/取消;
- 多批次发货时单号后缀 `-01`/`-02` 递增(与采购收货单规则一致,单号前缀 `CC`+日期);
- 发货单「已完成」事件驱动**库存出库(负数)**与销售订单已发货数量累加;
- 与来源单关联:`document_type`(如 `SalesOrder`/`TransferOrder`)、`document_id`(来源单 ID);
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`(本次发货数量)。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 发货单已生成,待审核 |
| 2 | 已同意 | 审核通过,待发货 |
| 3 | 已完成 | 发货完成(库存已出库) |
| 4 | 部分发货 | 部分明细已发,还有剩余 |
| 5 | 已拒绝 | 已拒绝 |
| 6 | 已取消 | 已取消 |

## 查询命令

### dn-list [--page N] [--size N] [过滤字段=值 ...]

发货单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`customer_name`(客户名,模糊)、`warehouse_name`(仓库名,模糊)、`contact_phone`(联系电话,模糊)、`contact_name`(联系人,模糊)、`status`(状态,精确)、`warehouse_id`(仓库 ID,精确)、`target_warehouse_id`(目标仓库 ID,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> dn-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> dn-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> dn-list order_no=CC2608061234-01
```

### dn-get <发货单ID>

按 ID 查询发货单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> dn-get dn-1
```

### dn-find-for <ID1,ID2,...>

按 ID 批量查询(逗号分隔),用于一次拉齐多个发货单。

## 关键约定

- 发货操作由 WebUI 完成。
- 发货单是「销售 → 发货 → 出库」链路与「调拨调出」的中间环节,「已完成」才代表真实出库。
- 状态 `3`(已完成)与销售 `3`(已发货)含义接近但不完全相同,对账时注意区分。
- 过滤字段受限,客户端已做白名单拦截。
