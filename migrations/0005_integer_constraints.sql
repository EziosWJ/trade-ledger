-- SQLite's INTEGER affinity still accepts REAL values. Keep cents and item
-- quantities integral at the database boundary for legacy and direct clients.
CREATE TRIGGER IF NOT EXISTS trades_integer_validate_insert
BEFORE INSERT ON trades
FOR EACH ROW
WHEN typeof(NEW.total_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'trade amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS trades_integer_validate_update
BEFORE UPDATE OF total_cents ON trades
FOR EACH ROW
WHEN typeof(NEW.total_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'trade amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS trade_items_integer_validate_insert
BEFORE INSERT ON trade_items
FOR EACH ROW
WHEN typeof(NEW.qty) <> 'integer' OR typeof(NEW.price_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'trade item values must be integers');
END;

CREATE TRIGGER IF NOT EXISTS trade_items_integer_validate_update
BEFORE UPDATE OF qty, price_cents ON trade_items
FOR EACH ROW
WHEN typeof(NEW.qty) <> 'integer' OR typeof(NEW.price_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'trade item values must be integers');
END;

CREATE TRIGGER IF NOT EXISTS payments_integer_validate_insert
BEFORE INSERT ON payments
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'payment amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS payments_integer_validate_update
BEFORE UPDATE OF amount_cents ON payments
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'payment amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS offsets_integer_validate_insert
BEFORE INSERT ON offsets
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'offset amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS offsets_integer_validate_update
BEFORE UPDATE OF amount_cents ON offsets
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'offset amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS offset_allocations_integer_validate_insert
BEFORE INSERT ON offset_allocations
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'offset allocation amount must be an integer');
END;

CREATE TRIGGER IF NOT EXISTS offset_allocations_integer_validate_update
BEFORE UPDATE OF amount_cents ON offset_allocations
FOR EACH ROW
WHEN typeof(NEW.amount_cents) <> 'integer'
BEGIN
  SELECT RAISE(ABORT, 'offset allocation amount must be an integer');
END;
