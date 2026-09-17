CREATE TABLE IF NOT EXISTS counterparties (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  phone TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  tags TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trades (
  id TEXT PRIMARY KEY,
  counterparty_id TEXT NOT NULL REFERENCES counterparties(id),
  type TEXT NOT NULL,
  direction TEXT NOT NULL,
  total_cents INTEGER NOT NULL CHECK (total_cents > 0),
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trades_counterparty ON trades(counterparty_id);

CREATE TABLE IF NOT EXISTS trade_items (
  id TEXT PRIMARY KEY,
  trade_id TEXT NOT NULL REFERENCES trades(id),
  name TEXT NOT NULL,
  qty INTEGER NOT NULL DEFAULT 1 CHECK (qty > 0),
  price_cents INTEGER NOT NULL CHECK (price_cents > 0)
);
CREATE INDEX IF NOT EXISTS idx_items_trade ON trade_items(trade_id);

CREATE TABLE IF NOT EXISTS payments (
  id TEXT PRIMARY KEY,
  trade_id TEXT NOT NULL REFERENCES trades(id),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  note TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payments_trade ON payments(trade_id);
