# 生产:业务指南


## 核心概念

- **生产工单(ProductionOrder)** = 按成品 BOM 组织生产的单据,由 production-service 管理。
- **生产 → 领料出库 / 成品入库是一条链**:
  - 领料出库走**发货单**(工单的 `delivery_note_id`);
  - 成品入库走**收货单**(工单的 `receipt_notes` ID 列表);
  - 生产完成后成品入库到目标仓库 `target_warehouse_id`。
- 工单单头为**成品**(`item_id/code/name`、`quantity` 计划数量、`produced_quantity` 已生产数量、`presentage` 进度);明细 `items` 为**物料需求**(领料)。

## 状态机

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待处理 | 工单已提交,待接单 |
| 2 | 已接单 | 已接单 |
| 3 | 生产中 | 生产中 |
| 4 | 已完成 | 已完成 |
| 5 | 已挂起 | 已挂起(暂停) |
| 6 | 已拒绝 | 已拒绝 |
| 7 | 已取消 | 已取消 |

## Agent 怎么做

本技能只提供**查询**;生产操作在 WebUI。Agent 的职责是**追踪与核对**:

1. **按状态筛工单**:`prd-list status=3 --size 20`(生产中)、`prd-list status=1`(待处理,提醒接单)。
2. **看工单明细**:`prd-get <工单ID>` → 对比 `quantity` 与 `produced_quantity`、看 `presentage` 进度。
3. **追踪领料出库**:按工单的 `delivery_note_id` 用 `dn-get` 查领料发货单是否已完成(库存扣减)。
4. **追踪成品入库**:按工单的 `receipt_notes` 用 `rn-find-for` 查成品入库;结合 `inv-*` 核对目标仓库成品库存。
5. **按成品/仓库过滤**:`prd-list name=成品名称`、`prd-list target_warehouse_id=<仓库ID>`。

## 关键约定

- 生产状态 `3`(生产中)与销售/发货的同数值含义不同,对账时先看模块。
- 工单 `order_type` 过滤在后端按字符串 LIKE 匹配,传数值即可。
- 按目标仓库过滤用 `target_warehouse_id`(成品入库地),按领料仓库过滤用 `warehouse_id`。
