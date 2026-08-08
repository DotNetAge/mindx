# 盘点:业务指南


## 核心概念

- **盘点单(InventoryCheckOrder)** = 核对仓库实物与账面库存的单据,由 inspection-service 管理。
- 盘点类型:`0` 全面盘点、`1` 部分盘点、`2` 循环盘点。
- 盘点仓库 `warehouse_id/name`;明细行含差异:`quantity`(账面库存)、`actual_qty`(实盘数量)、`diff_qty`(差异数量=实盘−账面)、`diff_reason`(差异原因)、`process_method`(处理方式)、`diff_amount`(差异金额)。
- **盘点是对账的核心工具**:实盘与账面不一致时,`diff_qty` 非 0 即为差异。

## 状态机

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待审核 | 盘点单已创建,待审核 |
| 2 | 盘点中 | 盘点进行中 |
| 3 | 已完成 | 盘点完成 |
| 4 | 已取消 | 已取消 |

## Agent 怎么做

本技能只提供**查询**;盘点操作在 WebUI。Agent 的职责是**追踪与核对**:

1. **按状态筛盘点单**:`ins-list status=2 --size 20`(盘点中)、`ins-list status=3`(已完成,可看结果)。
2. **看盘点结果**:`ins-get <盘点单ID>` → 关注 `diff_qty`/`diff_reason` 非 0 的明细行,即账实差异。
3. **与库存交叉核对**:对 `quantity` 与 `inv-warehouse-summary warehouse_id=<仓库ID> item_id=<物料ID>` 的库存数比对。
4. **按仓库定位**:`ins-list warehouse_id=<仓库ID>`、`ins-list warehouse_name=仓库名`。

## 关键约定

- **查列表务必带 `status`**:盘点列表接口对 `status` 无"未传则跳过"守卫(其他模块均有),不带时后端按 `Status=0` 过滤,结果恒空。
- 按盘点类型(`order_type`)或起止时间过滤在服务端未使用,客户端已拦截;需要按类型找单时改用全量列表后按字段人工筛选。
- 盘点状态 3 才代表盘点完成、结果可用。
