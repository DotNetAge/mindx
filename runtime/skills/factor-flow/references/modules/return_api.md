# return_api.py — 退货模块使用说明

退货模块查询命令,带 `ro-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> ro-<命令> [参数]
```

## 退货单是什么

退货单(ReturnOrder)记录客户退回的物料(来源多为销售退货),由 returning-service 提供查询:

- 状态流转:**待审核 → 已同意 → 已完成**;可拒绝/取消;
- 退货类型:`0` 质量问题、`1` 数量不符、`2` 包装损坏、`3` 发错货、`4` 滞销、`5` 换货;
- 退入仓库 `warehouse_id/name`,关联销售订单 `sale_order_id`;
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`、`supplier_id/name`。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 退货单已提交,待审核 |
| 2 | 已同意 | 审核通过 |
| 3 | 已完成 | 退货完成(通常已驱动退入/报废等处理) |
| 4 | 已拒绝 | 已拒绝 |
| 5 | 已取消 | 已取消 |

## 退货类型枚举

| 值 | 类型 |
|---|---|
| 0 | 质量问题 |
| 1 | 数量不符 |
| 2 | 包装损坏 |
| 3 | 发错货 |
| 4 | 滞销 |
| 5 | 换货 |

## 查询命令

### ro-list [--page N] [--size N] [过滤字段=值 ...]

退货单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`warehouse_name`(退入仓库名,模糊)、`customer_name`(客户名,模糊)、`creator_name`(申请人,模糊)、`qc_name`(质检人,模糊)、`status`(状态,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> ro-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ro-list status=2 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> ro-list customer_name=华为 --size 20
```

### ro-get <退货单ID>

按 ID 查询退货单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> ro-get ro-1
```

## 关键约定

- 退货发起/审核等写操作由 WebUI 完成。
- 退货「已完成」通常驱动退入仓库的后续处理(入库/报废),具体影响需结合场景核对。
- 过滤字段受限,客户端已做白名单拦截(如按退货类型 `return_type` 过滤在服务端未生效,故不支持)。
