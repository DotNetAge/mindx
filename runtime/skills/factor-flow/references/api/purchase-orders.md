# 采购单服务 API 存档(仅供底层信息查询,不用于调用)

> 本文件为 purchasing-service(端口 8082)裸 HTTP API 的存档说明,**仅供底层信息查询**。实际调用一律使用本技能的 python 指令(`./scripts/api_client/cli.py`)。涉及审计的接口位于 audit-service(端口 8080)。

## 通用约定

- 所有 `/purchase-orders` 接口需携带 `Authorization: Bearer <access_token>`(JWT)。
- **业务响应**:HTTP 恒 200,`{"code":0,"msg":"success","data":...}`;业务失败为 `code` 非 0(400/404/500)。
- **认证失败**:真实 HTTP 401,body 为 `{"error":"..."}`。
- 列表分页响应:`{"code":0,"data":[...],"total":N,"page":P,"size":S}`。

## 采购单实体字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 聚合 ID(UUID) |
| `order_no` | string | 单号,`CG`+YYMMDD+4 位随机,空则自动生成 |
| `supplier_id` / `supplier_name` | string | 供应商(必填) |
| `contact_phone` | string | 联系电话 |
| `warehouse_id` / `warehouse_name` | string | 仓库(warehouse_id 必填) |
| `delivery_address` | string | 收货地址 |
| `demand_date` | string | 需求日期(RFC3339) |
| `total_amount` | float64 | 总金额(由调用方计算) |
| `handler_id` / `handler_name` | string | 经手人(必填) |
| `ordered_at` | string | 下单时间(服务端生成) |
| `status` | int | 状态枚举,见下 |
| `auditor_id` / `auditor_name` / `audited_at` / `comments` | string | 审核信息 |
| `cancel_reason` / `cancelled_at` | string | 取消信息 |
| `completed_at` | string | 完成时间 |
| `receipt_notes` | []string | 关联入库单 ID 列表 |
| `attachments` | []string | 凭证附件路径列表 |
| `remarks` | string | 备注 |
| `items` | []PurchaseItem | 明细 |
| `created_at` / `updated_at` | string | 审计时间(框架填充) |

**状态枚举**:`1` 待审核 / `2` 已同意 / `3` 部分收货 / `4` 已收货 / `5` 已完成 / `6` 已拒绝 / `7` 已取消。

## 明细 PurchaseItem

| 字段 | 类型 | 说明 |
|---|---|---|
| `item_id` | string | 物料 ID(必填) |
| `code` / `name` | string | 编码 / 名称(name 必填) |
| `category` / `spec` / `color` / `unit` | string | 分类 / 规格 / 颜色 / 单位 |
| `price` | float64 | 采购单价 |
| `quantity` | float64 | 采购数量 |
| `received_qty` | float64 | 已收货数量(创建时通常为 0) |
| `remarks` | string | 行备注 |

## 接口一览

### 查询(GET)

| 路径 | 参数 | 说明 |
|---|---|---|
| `/purchase-orders/` | `page`、`size`、`order_no`、`status`、`supplier_name`、`handler_name`、`ordered_at`、`warehouse_id` | 列表(按下单时间倒序) |
| `/purchase-orders/:id` | 路径参数 | 详情 |
| `/purchase-orders/counting` | `status`(必填) | 按状态计数 |
| `/purchase-orders/find-for` | `ids`(逗号分隔) | 批量查询 |
| `/purchase-orders/:id/export` | 路径参数 | 导出 Excel 文件流 |

### 命令(POST)

| 路径 | 请求体关键字段 | 说明 |
|---|---|---|
| `/purchase-orders/prepare` | `id`(可选)、`supplier_id`*、`supplier_name`*、`warehouse_id`*、`warehouse_name`、`delivery_address`、`contact_phone`、`demand_date`、`total_amount`、`items[]`、`remarks`、`handler_id`*、`handler_name`* | 创建采购单(状态 → 1) |
| `/purchase-orders/approve` | `id`、`auditor_id`、`auditor_name`、`audited_at`、`comments` | 审核通过(状态 → 2) |
| `/purchase-orders/reject` | 同上 | 审核拒绝(状态 → 6) |
| `/purchase-orders/receive` | `id`、`arrived_at`、`items[]`(ReceiptItem) | 入库数量累加(部分/全部收货) |
| `/purchase-orders/complete` | `id`、`attachments[]` | 确认完成(状态 → 5) |
| `/purchase-orders/cancel` | `id`、`cancel_reason` | 取消(仅状态 ≤2;状态 → 7) |
| `/purchase-orders/assign` | `id`、`receipt_note_id` | 关联入库单(去重) |
| `/purchase-orders/upload` | multipart 字段 `file` | 上传凭证附件,返回 `{filename, path, url}` |

带 `*` 为后端 `validate:"required"` 字段。明细行 `item_id`、`name` 必填。

### 审计接口(audit-service,端口 8080)

| 路径 | 请求体 | 说明 |
|---|---|---|
| `/audit-logs/approve` | `document_type`、`document_id`、`auditor_id`、`auditor_name`、`comments` | 审批通过事件(采购单 `document_type=PurchaseOrder`) |
| `/audit-logs/reject` | 同上 | 审批拒绝事件 |

> 前端经 nginx 将 `/api/audit/` 重写为 `/audit-logs/`,因此 WebUI 调用的是 `/audit/approve`、`/audit/reject`,实际即上述接口。审核事件异步驱动采购单状态变更。

## 跨服务协作(事件驱动)

- **发布**:`PurchaseOrder` 事件流(Prepared/Approved/Rejected/Received/Completed/Cancelled/Assigned),供收货单等订阅。
- **订阅**:
  - 审计服务 `AuditLog.Approved/Rejected`(document_type=PurchaseOrder)→ 执行审核。
  - 收货服务 `ReceiptNote.Prepared` → 关联入库单;`ReceiptNote.Completed` → 累加入库数量并更新收货状态。
