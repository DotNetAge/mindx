# sales_api.py — 销售模块使用说明

销售模块查询命令,带 `so-` 前缀。

```
python ./scripts/api_client/cli.py --token <访问令牌> so-<命令> [参数]
```

## 销售订单是什么

销售订单(SalesOrder)记录客户销售与发货安排,由 sales-service 提供查询:

- 状态流转:**待发货 → 已部分发货/已发货 → 已完成**(可取消);
- 发货由发货单(delivery-note)驱动:发货单「已完成」时销售订单明细的已发货数量累加,全部发完置「已发货」,部分发出置「已部分发货」;
- 订单明细字段:`item_id`、`code/name`、`spec/color/unit`、`price`(售价)、`quantity`(销售数量)、`actual_delivered`(实际发货数量)。

## 状态枚举

| 值 | 状态 | 说明 |
|---|---|---|
| 1 | 待发货 | 订单已确认,尚未发货 |
| 2 | 已部分发货 | 部分明细已发,还有剩余 |
| 3 | 已发货 | 全部发货完成 |
| 4 | 已完成 | 订单完成 |
| 5 | 已取消 | 已取消 |

## 查询命令

### so-list [--page N] [--size N] [过滤字段=值 ...]

销售订单列表,返回 `{data, total, page, size}`。

支持过滤字段:`order_no`(单号,模糊)、`customer_name`(客户名,模糊)、`handler_name`(经手人,模糊)、`receiver`(收货人,模糊)、`receiver_phone`(收货电话,模糊)、`ordered_at`(下单日期,精确)、`status`(状态,精确)。

```
python ./scripts/api_client/cli.py --token <访问令牌> so-list --size 20
python ./scripts/api_client/cli.py --token <访问令牌> so-list customer_name=华为 status=3 --size 20
python ./scripts/api_client/cli.py --token <访问令牌> so-list order_no=XS2608061234
```

### so-get <销售订单ID>

按 ID 查询销售订单详情(含 items 明细)。

```
python ./scripts/api_client/cli.py --token <访问令牌> so-get so-1
```

### so-track <销售订单ID或XS单号>

**追踪发货(链路审计,方法论见 guides/tracking.md)**:一条命令返回——

- 销售订单基本信息与状态(含关联发货单 ID);
- 每张关联发货单:单号/状态/发货仓库/发货日期/明细;
- 每个物料:`quantity`(应发)、`delivered_qty`(已发)、`remaining_qty`(剩余应发)、`match`(已发齐/部分发货/未发货/超额发货);
- `checks` **检查点报告(核心,供 LLM 归因)**:每个检查点含 `check`/`ok`/`detail`/`guide`;
- `summary`:已发齐/部分/未发/超额项数、`pass_count`/`fail_count` 与 `conclusion`。

```
python ./scripts/api_client/cli.py --token <访问令牌> so-track XS2608061234
python ./scripts/api_client/cli.py --token <访问令牌> so-track so-1
```

**检查点列表**与各检查点的失败导引词见 `guides/tracking.md`(so-track 一节)。

## 关键约定

- 销售下单、发货等写操作由 WebUI 完成。
- 状态 `3`(已发货)是销售→发货→出库链路的中间态,追踪发货用 `so-track`,核对库存用 `inv-*`。
- 过滤字段受限,客户端已做白名单拦截;不支持的字段会被提示。
