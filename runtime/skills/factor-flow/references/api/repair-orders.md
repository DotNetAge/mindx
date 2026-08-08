# 维修单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 repairing-service(端口 8091)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`rp-*`)。

## 通用约定

- 所有 `/repair-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 维修单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号 |
| `status` | int | 状态:1 待审核、2 已同意、3 维修中、4 已完成、5 已拒绝、6 已取消 |
| `repaired_at` / `completed_at` | time | 维修日期 / 完成时间 |
| `reason` | string | 维修原因 |
| `cost` | float64 | 维修成本 |
| `treatment` | string | 处理方式 |
| `warehouse_id` / `warehouse_name` | string | 仓库 |
| `handler_id` / `handler_name` | string | 处理人 |
| `auditor_id` / `auditor_name` | string | 审核人 |
| `repairer_id` / `repairer_name` | string | 维修人 |
| `items` | []RepairItem | 维修明细 |

## 明细 RepairItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `warehouse_id` / `warehouse_name` | string | 仓库 |
| `item_id` / `item_type` | string / int | 物料 ID / 类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 数量 |
| `repair_cost` | float64 | 维修成本 |
| `fault_description` | string | 故障描述 |
| `repair_plan` | string | 维修方案 |
| `repair_result` | string | 维修结果 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/repair-orders/` | `page`、`size`(必传)、`order_no`、`warehouse_id`、`warehouse_name`、`status` | 列表 |
| `/repair-orders/:id` | 路径参数 | 详情 |
