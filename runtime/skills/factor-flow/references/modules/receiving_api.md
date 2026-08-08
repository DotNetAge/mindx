# receiving_api.py — 收货模块使用说明

收货入库模块查询命令,带 `rn-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> rn-<命令> [参数]
```

## 收货单是什么

收货单(ReceiptNote)由采购单「已同意」后经 receiving-service **自动生成**:

- 单号规则:采购单 `CG...` → 收货单 `JC...`(多批次收货时后缀 `-01`/`-02` 递增);
- 与采购单的关联:`document_type` = `PurchaseOrder`,`document_id` = 采购单 ID;
- 采购单详情的 `receipt_notes` 数组直接存有关联收货单 ID(**追踪时用它反查**,收货单列表接口不支持按 document_id 过滤);
- 收货单「已完成」事件驱动**库存增加**与采购单 `received_qty` 更新。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 草稿/准备中 | 采购单同意后自动生成,列表默认不可见 |
| 2 | 待收货 | 开始(Start)后可见 |
| 3 | 已完成 | 确认(Confirm)收货完成 |
| 4 | 已取消 | 已取消 |

## 查询命令

### rn-list [--page N] [--size N] [过滤字段=值 ...]

收货单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(收货单号)、`warehouse_name`(仓库名,模糊)、`warehouse_id`、`source_warehouse_id`(来源仓,采购场景为供应商 ID)、`document_type`(来源类型,如 `PurchaseOrder`)、`status`(状态,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> rn-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> rn-list document_type=PurchaseOrder status=3 --size 50
python ./scripts/api_client/cli.py --token <访问令牌> rn-list order_no=JC2608061234-01
```

### rn-get <收货单ID>

按 ID 查询收货单详情(含 items 明细)。明细行字段:`item_id`、`item_type`、`supplier_id/name`、`code`、`name`、`category`、`spec`、`color`、`unit`、`price`、`quantity`(收货数量)、`remarks`。

```
python ./scripts/api_client/cli.py --token <访问令牌> rn-get rn-1
```

### rn-find-for <ID1,ID2,...>

按 ID 批量查询(逗号分隔)。`po-track` 内部就是用采购单的 `receipt_notes` ID 列表走此接口一次拉齐。

## 关键约定

- **收货是采购的下游**:收货单由系统自动生成,本模块不提供创建/确认操作(WebUI 操作)。
- **追踪到货情况优先用 `po-track`**(一条命令给出收货单 + 库存核对 + 剩余量),需要收货单原始数据时再用 `rn-*`。
