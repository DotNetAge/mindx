# 收货单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 receiving-service(端口 8086)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`,`rn-*` / `po-track`)。

## 通用约定

- 所有 `/receipt-notes` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 收货单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号,`JC`+YYMMDD+4 位随机;多批次收货时后缀 `-01`/`-02` 递增 |
| `document_type` | string | 来源类型:`PurchaseOrder` / `SalesOrder` / `TransferOrder` / `ProductionOrder` / `DeliveryNote` |
| `document_id` | string | **来源单 ID**(采购场景存采购单 ID) |
| `warehouse_id` / `warehouse_name` | string | 接收仓 |
| `source_warehouse_id` / `source_warehouse_name` | string | 来源仓(采购场景存供应商 ID/名称) |
| `handler_id` / `handler_name` | string | 经手人 |
| `receipted_at` | string | 入库(确认收货)时间 |
| `remarks` | string | 备注 |
| `cancel_reason` / `cancelled_at` | string | 取消信息 |
| `attachments` | []string | 凭证附件 |
| `items` | []ReceiptItem | 明细 |
| `status` | int | 状态:1 草稿/准备中、2 待收货、3 已完成、4 已取消 |

## 明细 ReceiptItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `item_id` | string | 物料 ID |
| `item_type` | int | 物料类型(采购 0 / 生产 1) |
| `supplier_id` / `supplier_name` | string | 供应商 |
| `code` / `name` | string | 编码 / 名称 |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` | float64 | 采购单价 |
| `quantity` | float64 | **收货数量** |
| `remarks` | string | 行备注 |

## 接口一览

### 查询(GET)

| 路径 | 参数 | 说明 |
|---|---|---|
| `/receipt-notes/` | `page`、`size`(必传)、`order_no`、`warehouse_name`、`warehouse_id`、`source_warehouse_id`、`document_type`、`status` | 列表(**不支持按 document_id 过滤**) |
| `/receipt-notes/:id` | 路径参数 | 详情 |
| `/receipt-notes/find-for` | `ids`(逗号分隔) | 批量查询 |
| `/receipt-notes/:id/export` | 路径参数 | 导出 Excel 文件流 |

### 命令(POST)

| 路径 | 请求体关键字段 | 说明 |
|---|---|---|
| `/receipt-notes/prepare` | `order_no`、`document_type`、`document_id`、`warehouse_id/name`、`source_warehouse_id/name`、`items[]`、`remarks` | 准备入库单(状态 → 1) |
| `/receipt-notes/start` | `id` | 开始入库(状态 → 2,列表可见) |
| `/receipt-notes/confirm` | `id`、`handler_id/name`、`attachments[]`、`items[]`(ConfirmItem) | 确认收货(状态 → 3);**部分收货时自动生成下一批收货单**(单号后缀递增) |
| `/receipt-notes/cancel` | `id`、`cancel_reason` | 取消(状态 → 4) |
| `/receipt-notes/upload` | multipart 字段 `file` | 上传凭证附件 |

## 跨服务协作(事件驱动)

- **发布**:`ReceiptNote` 事件流(Prepared/Started/Completed/Cancelled)。
- **订阅**:
  - purchasing-service 的 `PurchaseOrder.Prepared` → 自动 Prepare 收货单(单号 `CG→JC` 替换 + `-01`,ItemType=0,Quantity=采购数量)。
  - **`ReceiptNote.Completed` → 采购单 `received_qty` 累加 + 状态更新(部分收货/已收货)+ stock-service 库存增加(`PushItem +Quantity`)**。
