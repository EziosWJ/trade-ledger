# API（手写契约，V1）

金额单位：分（`amount_cents` / `total_cents`，`> 0` 的整数）。展示层除以 100。

鉴权：未设置 `TRADE_LEDGER_PASSWORD` 时免鉴权；设置后 `/api/*` 需
`Authorization: Bearer <pw>` 或 `X-API-Token: <pw>`。

## 往来对象

- `POST /api/counterparties` `{name, phone?, note?, tags?}` → 201 Counterparty
  - `tags` 子集：`customer/supplier/peer/individual`
- `GET /api/counterparties` → Counterparty[]
  - 每个对象包含全量未结清余额：`receivable_cents`（应收）和 `payable_cents`（应付）
- `GET /api/counterparties/{id}/timeline` → TimelineEntry[]（业务+结算全局倒序混排；不存在的对象返回 404）

## 业务

- `POST /api/trades` `{counterparty_id, type, direction?, total_cents, note?, items?}` → 201 Trade
  - `type`: `sale/service/purchase/transfer`
  - `direction`: `in`（应收）/`out`（应付）；`sale/service` 默认 `in`，`purchase` 默认 `out`，`transfer` 必填方向
  - `items` 可空；为空即一口价。每项为 `{name, qty, price_cents}`，名称非空、数量为正整数、单价为正整数分；`total_cents` 仍是结算本金。
- `GET /api/trades?counterparty_id=` → Trade[]（含 `paid_cents` / `remaining_cents` / `items`）

## 结算（V1 Payment 与 Offset）

- `POST /api/trades/{id}/payments` `{amount_cents, note?}` → 201 `{paid_cents}`
  - 累计超 `total_cents` 返回 400；一笔业务可多次结算

- `POST /api/offsets` `{receivable_trade_id, payable_trade_id, amount_cents, note?}` → 201 Offset
  - 两笔业务必须属于同一往来对象，且分别为未结应收和未结应付；抵扣金额不得超过任一方剩余金额。
  - 双边各生成一条结算记录并共享 `offset_id`，原始业务不会被覆盖。
  - 响应包含 `receivable_remaining_cents` 和 `payable_remaining_cents`。

## 概览

- `GET /api/overview` → `{receivable_cents, payable_cents, recent[20]}`
  - 两个余额字段汇总全部未结业务，与 `recent` 的 20 条展示窗口无关。
- `GET /healthz` → `{status: ok}`（免鉴权）
