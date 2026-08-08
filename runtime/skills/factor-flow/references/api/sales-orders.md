# 销售订单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 sales-service(端口 8083)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`so-*`)。

## 通用约定

- 所有 `/sales-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 销售订单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 订单编号(`XS`+日期+序号) |
| `customer_id` / `customer_name` | string | 客户 ID / 名称 |
| `receiver` / `receiver_phone` | string | 收货人姓名 / 电话 |
| `delivery_address` | string | 收货地址 |
| `ordered_at` | time | 订单日期 |
| `scheduled_shipping_at` | time | 预计发货日期 |
| `total_amount` | float64 | 订单总金额 |
| `status` | int | 状态:1 待发货、2 已部分发货、3 已发货、4 已完成、5 已取消 |
| `handler_id` / `handler_name` | string | 经手人 |
| `warehouse_id` / `warehouse_name` | string | 发货仓库 |
| `delivery_notes` | []string | 关联发货单 ID 列表 |
| `items` | []SalesItem | 销售明细 |

## 明细 SalesItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `stock_id` | string | 库存 ID |
| `item_id` / `code` / `name` | string | 物料 ID / 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` | float64 | 售价 |
| `quantity` | float64 | 销售数量 |
| `actual_delivered` | float64 | **实际发货数量** |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/sales-orders/` | `page`、`size`(必传)、`order_no`、`customer_name`、`handler_name`、`receiver`、`receiver_phone`、`ordered_at`、`status` | 列表(过滤字段多为模糊 LIKE;`ordered_at`/`status` 为精确) |
| `/sales-orders/:id` | 路径参数 | 详情 |

## 跨服务协作(事件驱动)

- **订阅**:delivery-service 的 `DeliveryNote.Completed` → 销售订单明细 `actual_delivered` 累加;全部发完置「已发货」,部分发出置「已部分发货」;同时 stock-service 出库。
