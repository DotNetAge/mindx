# 发货单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 delivery-service(端口 8085)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`dn-*`)。

## 通用约定

- 所有 `/delivery-notes` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 发货单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号,`CC`+日期+序号;多批次发货时后缀 `-01`/`-02` 递增 |
| `document_type` / `document_id` | string | 来源类型 / 来源单 ID(如 `SalesOrder` / `TransferOrder` / `ProductionOrder`) |
| `warehouse_id` / `warehouse_name` | string | 发货仓库 |
| `target_warehouse_id` / `target_warehouse_name` | string | 目标仓库 |
| `customer_id` / `customer_name` | string | 客户 |
| `contact_name` / `contact_phone` | string | 联系人 / 电话 |
| `delivery_address` | string | 配送地址 |
| `deliveried_at` | time | 出库日期 |
| `handler_id` / `handler_name` | string | 经手人 |
| `proposer_id` / `proposer_name` | string | 发起人 |
| `status` | int | 状态:1 待审核、2 已同意、3 已完成、4 部分发货、5 已拒绝、6 已取消 |
| `items` | []InventoryItem | 库存明细(shared.InventoryItem) |

## 明细 InventoryItem(shared)

| 字段 | 类型 | 说明 |
|---|---|---|
| `stock_id` | string | 库存 ID |
| `item_id` / `item_type` | string / int | 物料 ID / 类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 发货数量 |
| `supplier_id` / `supplier_name` | string | 供应商 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/delivery-notes/` | `page`、`size`(必传)、`order_no`、`customer_name`、`warehouse_name`、`contact_phone`、`contact_name`、`status`、`warehouse_id`、`target_warehouse_id` | 列表 |
| `/delivery-notes/:id` | 路径参数 | 详情 |
| `/delivery-notes/find-for` | `ids`(逗号分隔) | 批量查询 |
| `/delivery-notes/:id/export` | 路径参数 | 导出 Excel 文件流 |

## 跨服务协作(事件驱动)

- **订阅**:采购/销售/调拨/生产等来源事件自动生成发货单。
- **发布**:`DeliveryNote.Completed` → **stock-service 库存出库(负数)** + 销售订单 `actual_delivered` 累加 + 调拨单「调出完成」。
