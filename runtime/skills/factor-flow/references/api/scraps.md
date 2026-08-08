# 报废单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 scraping-service(端口 8092)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`sc-*`)。注意路由前缀为 `/scraps`(非 `/scrap-orders`)。

## 通用约定

- 所有 `/scraps` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 报废单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号 |
| `scrap_type` | int | 报废类型:0 生产损耗、1 质量报废、2 积压报废、3 过期报废 |
| `warehouse_id` / `warehouse_name` | string | 仓库 |
| `status` | int | 状态:1 待审核、2 已同意、3 处理中、4 已完成、5 已拒绝、6 已取消 |
| `handler_id` / `handler_name` | string | 处理人 |
| `auditor_id` / `auditor_name` | string | 审核人 |
| `started_at` / `completed_at` | time | 处理开始/完成时间 |
| `qc_id` / `qc_name` | string | 质检员 |
| `treatment` / `result` | string | 处理方式 / 处理结果 |
| `total_amount` | float64 | 报废总金额 |
| `items` | []ScrapItem | 报废明细 |

## 明细 ScrapItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 库存 ID |
| `item_id` / `item_type` | string / int | 物料 ID / 类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 报废数量 |
| `amount` | float64 | 报废金额 |
| `reason` | string | 报废原因 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/scraps/` | `page`、`size`(必传)、`order_no`、`status`、`warehouse_id` | 列表 |
| `/scraps/:id` | 路径参数 | 详情 |

## 跨服务协作(事件驱动)

- **发布**:`ScrapOrder.Completed` → **stock-service 库存出库(负数)**。
