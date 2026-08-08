# 盘点单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 inspection-service(端口 8089)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`ins-*`)。注意路由前缀为 `/inspections`(非 `/inventory-check-orders`)。

## 通用约定

- 所有 `/inspections` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 盘点单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号 |
| `order_type` | int | 盘点类型:0 全面盘点、1 部分盘点、2 循环盘点 |
| `status` | int | 状态:1 待审核、2 盘点中、3 已完成、4 已取消 |
| `method` | int | 盘点方式 |
| `cycle` | int | 盘点周期:0 每日、1 每周、2 每月、3 每季度、4 每年 |
| `warehouse_id` / `warehouse_name` | string | 盘点仓库 |
| `scheduled_start_time` / `scheduled_end_time` | time | 预计开始/完成时间 |
| `started_at` / `completed_at` | time | 实际开始/完成时间 |
| `handler_id` / `handler_name` | string | 经手人 |
| `checker_id` / `checker_name` | string | 盘点人 |
| `confirmer_id` / `confirmer_name` | string | 确认人 |
| `items` | []InventoryCheckItem | 盘点明细 |

## 明细 InventoryCheckItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `item_id` / `item_type` | string / int | 物料 ID / 类型(1 物料、2 产品) |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / **账面库存数量** |
| `actual_qty` | float64 | **实盘数量** |
| `diff_qty` | float64 | **差异数量(实盘−账面)** |
| `diff_reason` | string | 差异原因 |
| `process_method` | string | 处理方式 |
| `diff_amount` | float64 | 差异金额 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/inspections/` | `page`、`size`(必传)、`order_no`、`warehouse_name`、`warehouse_id`、`status` | 列表(注:`order_type`/`started_at`/`completed_at` 在服务端未使用) |
| `/inspections/:id` | 路径参数 | 详情 |

## 跨服务协作(事件驱动)

- **发布**:盘点完成事件;盘点差异结果供账实核对(与 stock-service 库存数据交叉比对)。
