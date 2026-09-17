-- SQLite cannot add CHECK constraints to an existing table with ALTER TABLE.
-- These idempotent guards keep legacy databases aligned with the Trade domain.
CREATE TRIGGER IF NOT EXISTS trades_validate_insert
BEFORE INSERT ON trades
FOR EACH ROW
WHEN NEW.type NOT IN ('sale', 'service', 'purchase', 'transfer')
  OR (NEW.type IN ('sale', 'service') AND NEW.direction <> 'in')
  OR (NEW.type = 'purchase' AND NEW.direction <> 'out')
  OR (NEW.type = 'transfer' AND NEW.direction NOT IN ('in', 'out'))
BEGIN
  SELECT RAISE(ABORT, 'invalid trade type or direction');
END;

CREATE TRIGGER IF NOT EXISTS trades_validate_update
BEFORE UPDATE OF type, direction ON trades
FOR EACH ROW
WHEN NEW.type NOT IN ('sale', 'service', 'purchase', 'transfer')
  OR (NEW.type IN ('sale', 'service') AND NEW.direction <> 'in')
  OR (NEW.type = 'purchase' AND NEW.direction <> 'out')
  OR (NEW.type = 'transfer' AND NEW.direction NOT IN ('in', 'out'))
BEGIN
  SELECT RAISE(ABORT, 'invalid trade type or direction');
END;
