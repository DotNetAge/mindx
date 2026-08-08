# 库存:业务指南


## 核心概念

- **库存(Inventory)** = stock-service 的 CQRS 读模型。**每次入库是一条独立库存记录**,按"产品ID + 供应商ID + 仓库ID + 原单号 + 批次ID"唯一标识。
- **批次号(batch_id)**:每条库存记录都带,由入库当日日期生成(如 2026-08-06 → `20260806`)。**它只是记录的一个展示字段,不是查询维度**——`inv-list` 的 `batch_id` 过滤在后端未启用,三个 summary 视图也不含批次维度。
- **库存怎么变**:收货单「已完成」→ 库存增加(入库为正);出库(生产领料/销售发货)为负。
- **当前库存是存量**:历史累计,可能因领用/出库而减少,不等于"某张单的入库量";系统**无库存流水接口**,无法按单据回放变动明细。

## Agent 怎么做

1. **核对采购到货后的库存**:优先 `po-track <采购单ID或CG单号>` —— 自动按收货仓库 + 物料查库存并给出"库存是否覆盖已收量"的核对结论。
2. **手动查某仓库某物料库存**:`inv-warehouse-summary warehouse_id=<仓库ID> item_id=<物料ID>`(精确过滤、Quantity>0、按库存降序)。
3. **跨仓库查物料总量**:`inv-sku-summary item_id=<物料ID>`(item_id 模糊匹配)。
4. **按供应商/仓库看库存分布**:`inv-supplier-summary supplier_id=<供应商ID> [warehouse_id=<仓库ID>]`。
5. **看物理库存明细记录**:`inv-list warehouse_id=<仓库ID>` 或 `inv-list code=<编码>`。

## 关键约定

- 核对某物料库存**必须用 warehouse-summary**(支持 item_id 精确匹配);`inv-list` 不支持 item_id 过滤。
- 三个 summary 视图都强制 `Quantity > 0`:查不到记录 = 该仓库该物料当前无库存。
- 存量与入库量是两回事:采购数量与"库存增加"的一致性,以 `po-track` 的「已收量 vs 采购量 + 库存覆盖已收量」为准。
