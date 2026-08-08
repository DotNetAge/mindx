# scrap_api.py — 报废模块使用说明

报废模块查询命令,带 `sc-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> sc-<命令> [参数]
```

## 报废单是什么

报废单(ScrapOrder)记录物料报废(生产损耗/质量报废/积压/过期等),由 scraping-service 提供查询:

- 状态流转:**待审核 → 已同意 → 处理中 → 已完成**;可拒绝/取消;
- 报废类型:`0` 生产损耗、`1` 质量报废、`2` 积压报废、`3` 过期报废;
- 报废仓库 `warehouse_id/name`,处理方式 `treatment`,处理结果 `result`;
- 报废「已完成」通常驱动库存出库(负数);
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`、`amount`(报废金额)、`reason`(报废原因)。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 报废单已提交,待审核 |
| 2 | 已同意 | 审核通过 |
| 3 | 处理中 | 报废处理中 |
| 4 | 已完成 | 报废完成(库存已扣减) |
| 5 | 已拒绝 | 已拒绝 |
| 6 | 已取消 | 已取消 |

## 报废类型枚举

| 值 | 类型 |
|---|---|
| 0 | 生产损耗 |
| 1 | 质量报废 |
| 2 | 积压报废 |
| 3 | 过期报废 |

## 查询命令

### sc-list [--page N] [--size N] [过滤字段=值 ...]

报废单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`status`(状态,精确)、`warehouse_id`(仓库 ID,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> sc-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> sc-list status=4 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> sc-list warehouse_id=wh-1 --size 20
```

### sc-get <报废单ID>

按 ID 查询报废单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> sc-get sc-1
```

## 关键约定

- 报废发起/审核等写操作由 WebUI 完成。
- 状态 `4`(已完成)才代表报废生效、库存已扣减;对账库存时与 `inv-*` 交叉核对。
- 过滤字段受限,客户端已做白名单拦截。
