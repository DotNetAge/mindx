# 退货单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 returning-service(端口 8087)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`ro-*`)。

## 通用约定

- 所有 `/return-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 退货单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号 |
| `sale_order_id` | string | 关联销售订单 ID |
| `return_date` | time | 退货日期 |
| `return_type` | int | 退货类型:0 质量问题、1 数量不符、2 包装损坏、3 发错货、4 滞销、5 换货 |
| `status` | int | 状态:1 待审核、2 已同意、3 已完成、4 已拒绝、5 已取消 |
| `customer_id` / `customer_name` | string | 客户 |
| `reason` | string | 退货原因 |
| `amount` | float64 | 退货金额 |
| `warehouse_id` / `warehouse_name` | string | 退入仓库 |
| `logistics_type` / `tracking_no` | string | 物流方式 / 物流单号 |
| `handler_id` / `handler_name` | string | 申请人 |
| `auditor_id` / `auditor_name` | string | 审核人 |
| `qc_id` / `qc_name` | string | 质检人 |
| `items` | []ReturnItem | 退货明细 |

## 明细 ReturnItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 库存 ID |
| `item_id` / `item_type` | string / int | 物料 ID / 类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 退货数量 |
| `supplier_id` / `supplier_name` | string | 供应商 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/return-orders/` | `page`、`size`(必传)、`order_no`、`warehouse_name`、`customer_name`、`creator_name`、`qc_name`、`status` | 列表(注:`return_type`/`return_date` 在服务端已注释未生效) |
| `/return-orders/:id` | 路径参数 | 详情 |

## 跨服务协作(事件驱动)

- **订阅/发布**:退货完成事件通常驱动退入仓库的后续处理(入库/报废等)。
