# 调拨单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 transfer-service(端口 8090)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`tr-*`)。

## 通用约定

- 所有 `/transfer-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 调拨单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号 |
| `source_warehouse_id` / `source_warehouse_name` | string | 调出仓库 |
| `target_warehouse_id` / `target_warehouse_name` | string | 调入仓库 |
| `transfer_type` | int | 调拨类型:0 正常调拨、1 紧急调拨、2 库存平衡、3 生产需求 |
| `status` | int | 状态:1 待审核、2 已同意、3 调出完成、4 调入开始、5 已完成、6 已拒绝、7 已取消 |
| `reason` | string | 调拨原因 |
| `handler_id` / `handler_name` | string | 申请人 |
| `auditor_id` / `auditor_name` | string | 审核人 |
| `estimated_arrival_at` / `actual_arrival_at` | time | 预计/实际到货日期 |
| `carrier` / `logistics_no` | string | 承运商 / 物流单号 |
| `transportation_cost` / `total_amount` | float64 | 运输费用 / 调拨总金额 |
| `delivery_note_id` | string | **调出仓出库单 ID** |
| `receipt_note_id` | string | **调入仓入库单 ID** |
| `items` | []TransferItem | 调拨明细 |

## 明细 TransferItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `item_id` / `item_type` | string / int | 物料 ID / 类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 调拨数量 |
| `supplier_id` / `supplier_name` | string | 供应商 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/transfer-orders/` | `page`、`size`(必传)、`order_no`、`source_warehouse_name`、`source_warehouse_id`、`target_warehouse_name`、`target_warehouse_id`、`status`、`creator_name` | 列表(注:`type`/`created_at_start`/`created_at_end` 在服务端已注释未生效) |
| `/transfer-orders/:id` | 路径参数 | 详情 |

## 跨服务协作(事件驱动)

- **订阅/发布**:调出仓出库单(delivery-note)完成 → 「调出完成」;调入仓入库单(receipt-note)创建 → 「调入开始」,完成 → 「已完成」。
