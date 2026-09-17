CREATE TABLE IF NOT EXISTS offsets (
  id TEXT PRIMARY KEY,
  counterparty_id TEXT NOT NULL REFERENCES counterparties(id),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  note TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_offsets_counterparty ON offsets(counterparty_id);

CREATE TABLE IF NOT EXISTS offset_allocations (
  id TEXT PRIMARY KEY,
  offset_id TEXT NOT NULL REFERENCES offsets(id),
  trade_id TEXT NOT NULL REFERENCES trades(id),
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  created_at TEXT NOT NULL,
  UNIQUE (offset_id, trade_id)
);
CREATE INDEX IF NOT EXISTS idx_offset_allocations_trade ON offset_allocations(trade_id);
