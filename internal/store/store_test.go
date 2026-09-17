package store

import (
	"testing"

	_ "modernc.org/sqlite"

	"github.com/EziosWJ/trade-ledger/migrations"
)

func TestMigrateEnforcesTradeTypeAndDirection(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO counterparties (id, name, created_at) VALUES ('cp', '测试对象', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	for name, query := range map[string]string{
		"sale out":                   `INSERT INTO trades (id, counterparty_id, type, direction, total_cents, created_at) VALUES ('sale-out', 'cp', 'sale', 'out', 100, '2026-01-01T00:00:00Z')`,
		"unknown type":               `INSERT INTO trades (id, counterparty_id, type, direction, total_cents, created_at) VALUES ('unknown', 'cp', 'other', 'in', 100, '2026-01-01T00:00:00Z')`,
		"transfer missing direction": `INSERT INTO trades (id, counterparty_id, type, direction, total_cents, created_at) VALUES ('transfer-empty', 'cp', 'transfer', '', 100, '2026-01-01T00:00:00Z')`,
		"fractional trade amount":    `INSERT INTO trades (id, counterparty_id, type, direction, total_cents, created_at) VALUES ('fractional-trade', 'cp', 'sale', 'in', 100.5, '2026-01-01T00:00:00Z')`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.Exec(query); err == nil {
				t.Fatal("invalid trade should be rejected by migration guard")
			}
		})
	}
	if _, err := db.Exec(`INSERT INTO trades (id, counterparty_id, type, direction, total_cents, created_at) VALUES ('transfer-in', 'cp', 'transfer', 'in', 100, '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("valid transfer should be accepted: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO trade_items (id, trade_id, name, qty, price_cents) VALUES ('fractional-item', 'transfer-in', '网卡', 1.5, 100)`); err == nil {
		t.Fatal("fractional item quantity should be rejected by migration guard")
	}
}
