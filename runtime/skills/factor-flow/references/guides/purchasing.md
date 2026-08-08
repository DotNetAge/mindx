# 采购:业务指南


## 核心概念

- **采购单(PurchaseOrder)** = 向供应商下达的采购订单,含单头(供应商、仓库、经手人、需求日期、总金额)与明细 items(物料、单价、数量、已收货数量)。
- **单号**由系统生成:前缀 `CG` + 年月日 + 4 位随机数(如 `CG2608061234`)。
- **收货**由收货单(receipt-note)完成事件驱动:收货完成后采购单明细的已收货数量(`received_qty`)累加,同时增加收货仓库库存。
- **本技能只提供查询与追踪**;下单、审核、取消、上传凭证、确认完成等写操作均在 **WebUI** 完成。

## 采购流程(状态机,供查询时理解单据状态)

```
1 待审核 ──同意──▶ 2 已同意 ──收货──▶ 3 部分收货 / 4 已收货 ──完成──▶ 5 已完成
      │                            │
      └──拒绝──▶ 6 已拒绝          └──取消──▶ 7 已取消
```

| 值 | 状态 | 含义 |
|---|---|---|
| 1 | 待审核 | 已下单未审核,等待审批 |
| 2 | 已同意 | 审批通过,等待收货 |
| 3 | 部分收货 | 明细已部分到货 |
| 4 | 已收货 | 明细已全部到货 |
| 5 | 已完成 | 收货完成并确认 |
| 6 | 已拒绝 | 审核拒绝(查看拒绝原因) |
| 7 | 已取消 | 已取消(查看取消原因) |

> 收货状态变更由收货单完成事件驱动;审核由审计服务事件异步驱动,查询到状态与预期不符时,引导用户核对相关单据(详见 `tracking.md` 的归因方法)。

## 查询怎么用

```
python ./scripts/api_client/cli.py --token <访问令牌> po-list [过滤字段=值 ...] --size N
python ./scripts/api_client/cli.py --token <访问令牌> po-get <采购单ID>
python ./scripts/api_client/cli.py --token <访问令牌> po-counting --status N
python ./scripts/api_client/cli.py --token <访问令牌> po-find-for <ID1,ID2,...>
python ./scripts/api_client/cli.py --token <访问令牌> po-export <采购单ID> [--out 路径]
```

- 列表过滤字段:`order_no`(单号)、`status`(状态)、`supplier_name`(供应商)、`handler_name`(经手人)、`ordered_at`(下单日期)、`warehouse_id`(仓库)。
- `po-get` 详情中含 `items`(明细,含已收数量)与 `receipt_notes`(关联收货单 ID 列表)。
- 按状态分布看总量:`po-counting --status 1`(待审核)/ `2`(已同意)/ `4`(已收货)等,可用于汇报"有几张单待审核/未到货"。

## 追踪采购单(到货 -> 库存核对)

下完采购单后最重要的能力:**通过采购单号(CG 开头)追踪是否到货、库存是否正确、还差多少物料**。

```
python ./scripts/api_client/cli.py --token <访问令牌> po-track <采购单ID或CG单号>
```

一条命令完成四件事:

1. **是否到货**:列出关联收货单及各自状态/收货日期/收货数量。
2. **到货进度**:每物料对比 采购量 `quantity` vs 已收货量 `received_qty`,给出 剩余应到货量 `remaining_qty` 与状态(已到齐/部分到货/未到货/超额)。
3. **检查点报告**:`checks[]`(c2 到货进度 / c3 两口径一致性 / c4 终点对齐库存覆盖 / c5 收货单状态完整性)——程序判定 `ok` 与 `detail`,LLM 按 `guide` 归因。
4. **结论**:`summary.conclusion`(全部到货 / 部分到货仍差 N 项 / 尚未到货)与 `pass_count`/`fail_count`。

> 库存核对口径:当前库存是**存量**(历史累计),系统无流水接口,故核对以"已收量 = 采购量"与"库存覆盖已收量"为准,详见 `../modules/purchase_api.md` 的 po-track 说明。

示例:部分收货时,物料剩余量在 items 的 `remaining_qty`;向用户汇报时直接引用即可。存在 `ok=False` 的检查点时,按 `guide` 用 rn-get/rn-find-for/inv-warehouse-summary 取证后,向用户解释「哪张单、哪个环节、差多少、下一步」(方法论见 `../guides/tracking.md`)。

## 写操作说明

- 下单(单条/按 BOM 批量)、审核(同意/拒绝)、取消、上传凭证、确认完成均在 **WebUI** 完成,本技能不提供。
- Agent 查询到"待审核/未到货/库存不足"等情形时,**告知用户**去 WebUI 处理,或建议用户操作后重新查询确认。

## 关联主数据

| 数据 | 来源 | 用途 |
|---|---|---|
| 供应商与报价单 | `supplier-list` / `supplier-get` | 供应商名称/电话 |
| 仓库 | `warehouse-list` / `warehouse-get` | 仓库名称、收货地址 |
| 物料 | `material-list` / `material-get` / `material-find` | 明细的编码/规格/颜色/单位/分类 |
| 成品与 BOM | `catalog-list` / `material-bom` | BOM 展开与采购量核算 |
| 登录/账号 | authorization-account 技能 | 令牌 |
