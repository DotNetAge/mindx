# 库存服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 stock-service(端口 8084)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`inv-*` / `po-track`)。

## 通用约定

- 所有 `/inventories` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 库存记录字段(InventoryList)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID,每次入库一条新记录) |
| `warehouse_id` / `warehouse_name` | string | 仓库 |
| `batch_id` | int | 批次号 |
| `item_id` | string | 物料 ID |
| `item_type` | int | 物料类型 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` | float64 | 采购单价 |
| `quantity` | float64 | 数量(入库为正、出库为负) |
| `supplier_id` / `supplier_name` | string | 供应商 |
| `remarks` | string | 备注 |

## 接口一览(GET)

| 路径 | 过滤参数 | 说明 |
|---|---|---|
| `/inventories/` | `page`、`size`(必传)、`warehouse_id`、`warehouse_name`(模糊)、`batch_id`、`supplier_id`、`supplier_name`(模糊)、`code`(模糊)、`name`(模糊) | 物理库存列表(**无 item_id 过滤**) |
| `/inventories/:id` | 路径参数 | 单条库存记录详情 |
| `/inventories/warehouse-summary/` | `page`、`size`(必传)、`warehouse_id`(EQ)、`warehouse_name`(LIKE)、`item_id`(EQ)、`code`(LIKE)、`name`(LIKE) | **仓库库存摘要**:强制 Quantity>0,按 Quantity 降序 |
| `/inventories/sku-summary/` | `page`、`size`(必传)、`item_id`(LIKE)、`code`(LIKE)、`name`(LIKE) | SKU 摘要(跨仓库按物料),Quantity>0 |
| `/inventories/supplier-summary/` | `page`、`size`(必传)、`warehouse_id`、`warehouse_name`、`supplier_id`、`supplier_name`、`item_id`(EQ)、`code`、`name` | 供应商摘要,Quantity>0 |

> 三个 summary 视图均强制 `Quantity > 0`(零库存不返回)。

### 命令(POST)

| 路径 | 请求体关键字段 | 说明 |
|---|---|---|
| `/inventories/push` | 库存记录字段 | 幂等入库(内部事件驱动使用,手工调用受限) |

## 跨服务协作(事件驱动)

- **发布**:`Inventory` 事件流(入库/出库)。
- **订阅**:
  - `ReceiptNote.Completed` → **库存增加**(PushItem +Quantity,收货仓库维度)。
  - 出库(生产领料/销售发货等)→ PushItem 负数。
- **无库存流水/变动记录接口**:当前库存为存量,历史出入库明细无法按单查询。
