# 维修:业务指南


## 核心概念

- **维修单(RepairOrder)** = 记录不良品/故障品返修的单据,由 repairing-service 管理。
- 维修仓库 `warehouse_id/name`,维修成本 `cost`,处理方式 `treatment`。
- 明细行含故障上下文:`fault_description`(故障描述)、`repair_plan`(维修方案)、`repair_result`(维修结果)、`repair_cost`(行维修成本)。

## 状态机

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 维修单已提交,待审核 |
| 2 | 已同意 | 审核通过 |
| 3 | 维修中 | 维修中 |
| 4 | 已完成 | 维修完成 |
| 5 | 已拒绝 | 已拒绝 |
| 6 | 已取消 | 已取消 |

## Agent 怎么做

本技能只提供**查询**;维修操作在 WebUI。Agent 的职责是**追踪与核对**:

1. **按状态筛维修单**:`rp-list status=3 --size 20`(维修中)、`rp-list status=1`(待审核,提醒处理)。
2. **看维修明细**:`rp-get <维修单ID>` → 明细的故障/方案/结果字段,汇总 `cost`。
3. **按仓库定位**:`rp-list warehouse_id=<仓库ID>`、`rp-list warehouse_name=仓库名`。

## 关键约定

- 维修状态数值与销售/生产/发货不同(维修 3=维修中,发货 3=已完成),对账时先看模块。
- 过滤字段仅 4 个(order_no/warehouse_id/warehouse_name/status),其余字段无法按列表过滤。
