# production_api.py — 生产模块使用说明

生产模块查询命令,带 `prd-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> prd-<命令> [参数]
```

## 生产工单是什么

生产工单(ProductionOrder)按成品 BOM 组织生产,由 production-service 提供查询:

- 状态流转:**待处理 → 已接单 → 生产中 → 已完成**;可挂起/拒绝/取消;
- 工单可带目标仓库(`target_warehouse_id/name`,成品入库目标)与领料出库单(`delivery_note_id`)、入库单(`receipt_notes`);
- 明细 `items` 为物料库存需求(领料),单头 `item_id/code/name` 为成品,`quantity` 为计划生产数量,`produced_quantity` 为已生产数量,`presentage` 为生产进度。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待处理 | 工单已提交,待接单 |
| 2 | 已接单 | 已接单 |
| 3 | 生产中 | 生产中 |
| 4 | 已完成 | 已完成 |
| 5 | 已挂起 | 已挂起(暂停) |
| 6 | 已拒绝 | 已拒绝 |
| 7 | 已取消 | 已取消 |

## 工单类型枚举

| 值 | 类型 |
|---|---|
| 0 | 加工 |
| 1 | 包装 |
| 2 | 维修 |
| 3 | 其他 |

## 查询命令

### prd-list [--page N] [--size N] [过滤字段=值 ...]

生产工单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_type`(工单类型,模糊匹配,注意后端按字符串 LIKE 处理)、`status`(状态,精确)、`warehouse_name`(仓库名,模糊)、`name`(成品名称,模糊)、`handler_name`(处理人,模糊)、`warehouse_id`(仓库 ID,精确)、`target_warehouse_id`(目标仓库 ID,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> prd-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> prd-list status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> prd-list target_warehouse_id=wh-1 --size 20
```

### prd-get <生产工单ID>

按 ID 查询生产工单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> prd-get prd-1
```

### prd-find-for <ID1,ID2,...>

按 ID 批量查询(逗号分隔),用于一次拉齐多个工单(如按销售单关联的生产工单列表)。

## 关键约定

- 生产下单、接单、领料等写操作由 WebUI 完成。
- 生产 → 领料出库 / 成品入库:领料出库走发货单(`dn-*`,工单的 `delivery_note_id`),成品入库走收货单(`rn-*`,工单的 `receipt_notes`)。
- 状态 `3`(生产中)与销售/发货的同数值含义不同,对账时先看模块。
- 过滤字段受限,客户端已做白名单拦截。
