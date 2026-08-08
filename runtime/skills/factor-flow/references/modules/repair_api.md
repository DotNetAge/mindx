# repair_api.py — 维修模块使用说明

维修模块查询命令,带 `rp-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> rp-<命令> [参数]
```

## 维修单是什么

维修单(RepairOrder)记录不良品/故障品返修,由 repairing-service 提供查询:

- 状态流转:**待审核 → 已同意 → 维修中 → 已完成**;可拒绝/取消;
- 维修仓库 `warehouse_id/name`,维修成本 `cost`,处理方式 `treatment`;
- 明细 `items` 字段:`item_id`、`item_type`、`code/name`、`spec/color/unit`、`price`、`quantity`、`repair_cost`(维修成本)、`fault_description`(故障描述)、`repair_plan`(维修方案)、`repair_result`(维修结果)。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 维修单已提交,待审核 |
| 2 | 已同意 | 审核通过 |
| 3 | 维修中 | 维修中 |
| 4 | 已完成 | 维修完成 |
| 5 | 已拒绝 | 已拒绝 |
| 6 | 已取消 | 已取消 |

## 查询命令

### rp-list [--page N] [--size N] [过滤字段=值 ...]

维修单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`warehouse_id`(仓库 ID,精确)、`warehouse_name`(仓库名,模糊)、`status`(状态,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> rp-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> rp-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> rp-list warehouse_id=wh-1 --size 20
```

### rp-get <维修单ID>

按 ID 查询维修单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> rp-get rp-1
```

## 关键约定

- 维修发起等写操作由 WebUI 完成。
- 过滤字段受限,客户端已做白名单拦截。
