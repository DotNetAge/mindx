# 生产工单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 production-service(端口 8088)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`prd-*`)。

## 通用约定

- 所有 `/production-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 生产工单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 工单编号 |
| `order_type` | int | 工单类型:0 加工、1 包装、2 维修、3 其他 |
| `warehouse_id` / `warehouse_name` | string | 仓库(领料/加工地) |
| `target_warehouse_id` / `target_warehouse_name` | string | 目标仓库(成品入库地) |
| `item_id` / `code` / `name` | string | 成品 ID / 编码 / 名称 |
| `price` / `cost` | float64 | 单位价格 / 单位加工成本 |
| `quantity` / `produced_quantity` | float64 | 计划生产数量 / 已生产数量 |
| `presentage` | int | 生产进度 |
| `status` | int | 状态:1 待处理、2 已接单、3 生产中、4 已完成、5 已挂起、6 已拒绝、7 已取消 |
| `priority` | int | 优先级:0 高、1 中、2 低 |
| `manager_id` / `manager_name` | string | 负责人 |
| `handler_id` / `handler_name` | string | 处理人(发起人) |
| `delivery_note_id` | string | **领料出库单 ID** |
| `receipt_notes` | []string | **成品入库单 ID 列表** |
| `items` | []OrderItem | 物料库存需求明细 |

## 明细 OrderItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `stock_id` | string | 库存 ID |
| `item_id` / `item_type` | string / int | 物料 ID / 类型(0 原材料、1 中间产品、3 最终产品) |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` / `quantity` | float64 | 单价 / 需求数量 |
| `supplier_id` / `supplier_name` | string | 供应商 |

## 接口一览

| 路径 | 参数 | 说明 |
|---|---|---|
| `/production-orders/` | `page`、`size`(必传)、`order_type`(string LIKE)、`status`、`warehouse_name`、`name`、`handler_name`、`warehouse_id`、`target_warehouse_id` | 列表 |
| `/production-orders/:id` | 路径参数 | 详情 |
| `/production-orders/find-for` | `ids`(逗号分隔) | 批量查询 |

## 跨服务协作(事件驱动)

- **订阅/发布**:工单状态事件与领料(发货单)、成品入库(收货单)联动;`ProductionOrder` 事件流驱动物料领料出库与成品入库。
