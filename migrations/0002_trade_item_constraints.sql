-- Keep databases created before 0001 gained item checks safe as well.
CREATE TRIGGER IF NOT EXISTS trade_items_validate_insert
BEFORE INSERT ON trade_items
FOR EACH ROW
WHEN trim(NEW.name) = '' OR NEW.qty <= 0 OR NEW.price_cents <= 0
BEGIN
  SELECT RAISE(ABORT, 'invalid trade item');
END;

CREATE TRIGGER IF NOT EXISTS trade_items_validate_update
BEFORE UPDATE OF name, qty, price_cents ON trade_items
FOR EACH ROW
WHEN trim(NEW.name) = '' OR NEW.qty <= 0 OR NEW.price_cents <= 0
BEGIN
  SELECT RAISE(ABORT, 'invalid trade item');
END;
