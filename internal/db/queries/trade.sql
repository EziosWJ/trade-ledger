-- name: CreateCounterparty :exec
INSERT INTO counterparties (id, name, phone, note, tags, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListCounterparties :many
SELECT id, name, phone, note, tags, created_at
FROM counterparties
ORDER BY created_at DESC;

-- name: CounterpartyExists :one
SELECT EXISTS(SELECT 1 FROM counterparties WHERE id = ?) AS counterparty_exists;

-- name: ListCounterpartyBalances :many
SELECT
  t.counterparty_id,
  CAST(COALESCE(SUM(CASE
    WHEN t.direction = 'in' AND t.total_cents > COALESCE(p.paid_cents, 0)
    THEN t.total_cents - COALESCE(p.paid_cents, 0)
    ELSE 0
  END), 0) AS INTEGER) AS receivable_cents,
  CAST(COALESCE(SUM(CASE
    WHEN t.direction = 'out' AND t.total_cents > COALESCE(p.paid_cents, 0)
    THEN t.total_cents - COALESCE(p.paid_cents, 0)
    ELSE 0
  END), 0) AS INTEGER) AS payable_cents
FROM trades t
LEFT JOIN (
  SELECT trade_id, SUM(amount_cents) AS paid_cents
  FROM (
    SELECT trade_id, amount_cents FROM payments
    UNION ALL
    SELECT trade_id, amount_cents FROM offset_allocations
  ) settled
  GROUP BY trade_id
) p ON p.trade_id = t.id
GROUP BY t.counterparty_id;

-- name: CreateTrade :exec
INSERT INTO trades (id, counterparty_id, type, direction, total_cents, note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CreateTradeItem :exec
INSERT INTO trade_items (id, trade_id, name, qty, price_cents)
VALUES (?, ?, ?, ?, ?);

-- name: ListTradeItems :many
SELECT id, trade_id, name, qty, price_cents
FROM trade_items
WHERE trade_id = ?
ORDER BY rowid;

-- name: ListTrades :many
SELECT id, counterparty_id, type, direction, total_cents, note, created_at
FROM trades
ORDER BY created_at DESC;

-- name: ListTradesByCounterparty :many
SELECT id, counterparty_id, type, direction, total_cents, note, created_at
FROM trades
WHERE counterparty_id = ?
ORDER BY created_at DESC;

-- name: ListRecentTrades :many
SELECT id, counterparty_id, type, direction, total_cents, note, created_at
FROM trades
ORDER BY created_at DESC
LIMIT 20;

-- name: GetTradeTotal :one
SELECT total_cents
FROM trades
WHERE id = ?;

-- name: GetTrade :one
SELECT id, counterparty_id, type, direction, total_cents, note, created_at
FROM trades
WHERE id = ?;

-- name: SettlementTotalByTrade :one
SELECT CAST(COALESCE(SUM(amount_cents), 0) AS INTEGER) AS settled_cents
FROM (
  SELECT p.amount_cents FROM payments p WHERE p.trade_id = ?1
  UNION ALL
  SELECT a.amount_cents FROM offset_allocations a WHERE a.trade_id = ?1
) settled;

-- name: CreatePayment :exec
INSERT INTO payments (id, trade_id, amount_cents, note, occurred_at, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: OverviewBalances :one
SELECT
  CAST(COALESCE(SUM(CASE
    WHEN t.direction = 'in' AND t.total_cents > COALESCE(p.paid_cents, 0)
    THEN t.total_cents - COALESCE(p.paid_cents, 0)
    ELSE 0
  END), 0) AS INTEGER) AS receivable_cents,
  CAST(COALESCE(SUM(CASE
    WHEN t.direction = 'out' AND t.total_cents > COALESCE(p.paid_cents, 0)
    THEN t.total_cents - COALESCE(p.paid_cents, 0)
    ELSE 0
  END), 0) AS INTEGER) AS payable_cents
FROM trades t
LEFT JOIN (
  SELECT trade_id, SUM(amount_cents) AS paid_cents
  FROM (
    SELECT trade_id, amount_cents FROM payments
    UNION ALL
    SELECT trade_id, amount_cents FROM offset_allocations
  ) settled
  GROUP BY trade_id
) p ON p.trade_id = t.id;

-- name: CreateOffset :exec
INSERT INTO offsets (id, counterparty_id, amount_cents, note, occurred_at, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateOffsetAllocation :exec
INSERT INTO offset_allocations (id, offset_id, trade_id, amount_cents, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListTimelineEntries :many
WITH entries AS (
  SELECT 'trade' AS kind, t.id, t.id AS trade_id, t.total_cents AS amount_cents,
    CAST(t.type || CASE WHEN t.note = '' THEN '' ELSE ' ' || t.note END AS TEXT) AS note,
    t.created_at AS at, '' AS offset_id
  FROM trades t
  WHERE t.counterparty_id = sqlc.arg(counterparty_id)
  UNION ALL
  SELECT 'payment' AS kind, p.id, p.trade_id, p.amount_cents, p.note, p.occurred_at, '' AS offset_id
  FROM payments p
  JOIN trades t ON t.id = p.trade_id
  WHERE t.counterparty_id = sqlc.arg(counterparty_id)
  UNION ALL
  SELECT 'offset' AS kind, a.id, a.trade_id, a.amount_cents, o.note, o.occurred_at, o.id
  FROM offset_allocations a
  JOIN offsets o ON o.id = a.offset_id
  JOIN trades t ON t.id = a.trade_id
  WHERE t.counterparty_id = sqlc.arg(counterparty_id)
)
SELECT kind, id, trade_id, amount_cents, note, at, offset_id
FROM entries
ORDER BY at DESC, CASE kind WHEN 'payment' THEN 0 ELSE 1 END, id DESC;
