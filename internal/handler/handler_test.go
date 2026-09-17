package handler

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	"github.com/EziosWJ/trade-ledger/internal/service"
	"github.com/EziosWJ/trade-ledger/internal/store"
	"github.com/EziosWJ/trade-ledger/migrations"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return New(service.New(db), nil)
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// 首闭环 tracer：对象 → 销售 → 部分收款 → 时间线/概览，超收被拦截。
func TestTracer(t *testing.T) {
	h := testServer(t)

	rec := do(t, h, "POST", "/api/counterparties", map[string]any{"name": "老王电脑", "tags": []string{"customer"}})
	if rec.Code != 201 {
		t.Fatalf("create counterparty: %d %s", rec.Code, rec.Body.String())
	}
	var cp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&cp); err != nil {
		t.Fatal(err)
	}

	rec = do(t, h, "POST", "/api/trades", map[string]any{
		"counterparty_id": cp.ID, "type": "sale", "total_cents": 10000, "note": "装机两台",
	})
	if rec.Code != 201 {
		t.Fatalf("create trade: %d %s", rec.Code, rec.Body.String())
	}
	var tr struct {
		ID             string `json:"id"`
		RemainingCents int64  `json:"remaining_cents"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&tr); err != nil {
		t.Fatal(err)
	}

	rec = do(t, h, "POST", "/api/trades/"+tr.ID+"/payments", map[string]any{"amount_cents": 4000})
	if rec.Code != 201 {
		t.Fatalf("pay: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, "POST", "/api/trades/"+tr.ID+"/payments", map[string]any{"amount_cents": 7000})
	if rec.Code != 400 {
		t.Fatalf("overpay should be 400, got %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, "GET", "/api/trades?counterparty_id="+cp.ID, nil)
	var trades []service.Trade
	if err := json.NewDecoder(rec.Body).Decode(&trades); err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 || trades[0].RemainingCents != 6000 {
		t.Fatalf("remaining should be 6000, got %+v", trades)
	}

	rec = do(t, h, "GET", "/api/counterparties", nil)
	var counterparties []service.Counterparty
	if err := json.NewDecoder(rec.Body).Decode(&counterparties); err != nil {
		t.Fatal(err)
	}
	if len(counterparties) != 1 || counterparties[0].ReceivableCents != 6000 || counterparties[0].PayableCents != 0 {
		t.Fatalf("counterparty balance should be receivable 6000/payable 0, got %+v", counterparties)
	}

	rec = do(t, h, "GET", "/api/counterparties/"+cp.ID+"/timeline", nil)
	var tl []service.TimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&tl); err != nil {
		t.Fatal(err)
	}
	if len(tl) != 2 {
		t.Fatalf("timeline should have trade+payment, got %+v", tl)
	}

	rec = do(t, h, "GET", "/api/overview", nil)
	var ov service.Overview
	if err := json.NewDecoder(rec.Body).Decode(&ov); err != nil {
		t.Fatal(err)
	}
	if ov.ReceivableCents != 6000 {
		t.Fatalf("overview receivable should be 6000, got %+v", ov)
	}
}

func createTestCounterparty(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := do(t, h, "POST", "/api/counterparties", map[string]any{"name": "测试对象"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create counterparty: %d %s", rec.Code, rec.Body.String())
	}
	var c struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func createTestTrade(t *testing.T, h http.Handler, counterpartyID, typ string, total int64, direction string, items []service.TradeItem) string {
	t.Helper()
	body := map[string]any{
		"counterparty_id": counterpartyID,
		"type":            typ,
		"total_cents":     total,
		"items":           items,
	}
	if direction != "" {
		body["direction"] = direction
	}
	rec := do(t, h, "POST", "/api/trades", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create trade: %d %s", rec.Code, rec.Body.String())
	}
	var trade service.Trade
	if err := json.NewDecoder(rec.Body).Decode(&trade); err != nil {
		t.Fatal(err)
	}
	return trade.ID
}

func TestOverviewTotalsAreIndependentOfRecentWindow(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)
	for i := 0; i < 21; i++ {
		createTestTrade(t, h, cpID, "sale", 100, "", nil)
	}

	rec := do(t, h, "GET", "/api/overview", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body.String())
	}
	var overview service.Overview
	if err := json.NewDecoder(rec.Body).Decode(&overview); err != nil {
		t.Fatal(err)
	}
	if overview.ReceivableCents != 2100 {
		t.Fatalf("overview should include all 21 trades, got %d", overview.ReceivableCents)
	}
	if len(overview.Recent) != 20 {
		t.Fatalf("recent should contain at most 20 trades, got %d", len(overview.Recent))
	}
}

func TestCounterpartyFieldsAndTagsRoundTrip(t *testing.T) {
	h := testServer(t)
	rec := do(t, h, "POST", "/api/counterparties", map[string]any{
		"name": "综合对象", "phone": "13800000000", "note": "长期合作", "tags": []string{"customer", "supplier"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create counterparty: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "GET", "/api/counterparties", nil)
	var counterparties []service.Counterparty
	if err := json.NewDecoder(rec.Body).Decode(&counterparties); err != nil {
		t.Fatal(err)
	}
	if len(counterparties) != 1 || counterparties[0].Phone != "13800000000" || counterparties[0].Note != "长期合作" || len(counterparties[0].Tags) != 2 {
		t.Fatalf("counterparty fields should round trip, got %+v", counterparties)
	}
}

func TestTimelineGloballySortsTradesAndPayments(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)
	tradeA := createTestTrade(t, h, cpID, "sale", 1000, "", nil)
	tradeB := createTestTrade(t, h, cpID, "service", 2000, "", nil)

	rec := do(t, h, "POST", "/api/trades/"+tradeA+"/payments", map[string]any{"amount_cents": 100})
	if rec.Code != http.StatusCreated {
		t.Fatalf("payment: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "GET", "/api/counterparties/"+cpID+"/timeline", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("timeline: %d %s", rec.Code, rec.Body.String())
	}
	var timeline []service.TimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 3 || timeline[0].Kind != "payment" || timeline[0].TradeID != tradeA || timeline[1].ID != tradeB || timeline[2].ID != tradeA {
		t.Fatalf("timeline should be payment A, trade B, trade A; got %+v", timeline)
	}
}

func TestTimelineUnknownCounterpartyIsNotFound(t *testing.T) {
	h := testServer(t)
	rec := do(t, h, "GET", "/api/counterparties/missing/timeline", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown counterparty timeline should be 404, got %d", rec.Code)
	}
}

func TestOffsetSettlesBothDirectionsAndIsTraceable(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)
	receivableID := createTestTrade(t, h, cpID, "sale", 1000, "", nil)
	payableID := createTestTrade(t, h, cpID, "purchase", 800, "", nil)

	rec := do(t, h, "POST", "/api/offsets", map[string]any{
		"receivable_trade_id": receivableID,
		"payable_trade_id":    payableID,
		"amount_cents":        300,
		"note":                "同行抵扣",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("offset: %d %s", rec.Code, rec.Body.String())
	}
	var offset service.Offset
	if err := json.NewDecoder(rec.Body).Decode(&offset); err != nil {
		t.Fatal(err)
	}
	if offset.ReceivableRemainingCents != 700 || offset.PayableRemainingCents != 500 || offset.AmountCents != 300 {
		t.Fatalf("offset response has wrong remaining balances: %+v", offset)
	}

	rec = do(t, h, "GET", "/api/trades?counterparty_id="+cpID, nil)
	var trades []service.Trade
	if err := json.NewDecoder(rec.Body).Decode(&trades); err != nil {
		t.Fatal(err)
	}
	for _, trade := range trades {
		if trade.ID == receivableID && (trade.PaidCents != 300 || trade.RemainingCents != 700) {
			t.Fatalf("receivable should include offset settlement: %+v", trade)
		}
		if trade.ID == payableID && (trade.PaidCents != 300 || trade.RemainingCents != 500) {
			t.Fatalf("payable should include offset settlement: %+v", trade)
		}
	}

	rec = do(t, h, "GET", "/api/counterparties", nil)
	var counterparties []service.Counterparty
	if err := json.NewDecoder(rec.Body).Decode(&counterparties); err != nil {
		t.Fatal(err)
	}
	if len(counterparties) != 1 || counterparties[0].ReceivableCents != 700 || counterparties[0].PayableCents != 500 {
		t.Fatalf("counterparty balances should include offset, got %+v", counterparties)
	}

	rec = do(t, h, "GET", "/api/counterparties/"+cpID+"/timeline", nil)
	var timeline []service.TimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&timeline); err != nil {
		t.Fatal(err)
	}
	var offsetEntries []service.TimelineEntry
	for _, entry := range timeline {
		if entry.Kind == "offset" {
			offsetEntries = append(offsetEntries, entry)
		}
	}
	if len(offsetEntries) != 2 || offsetEntries[0].OffsetID == "" || offsetEntries[0].OffsetID != offsetEntries[1].OffsetID {
		t.Fatalf("offset should produce two linked timeline entries, got %+v", timeline)
	}

	rec = do(t, h, "POST", "/api/trades/"+receivableID+"/payments", map[string]any{"amount_cents": 700})
	if rec.Code != http.StatusCreated {
		t.Fatalf("payment after offset: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, "POST", "/api/trades/"+receivableID+"/payments", map[string]any{"amount_cents": 1})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("payment beyond offset-adjusted balance should be rejected, got %d", rec.Code)
	}
}

func TestOffsetRejectsInvalidPairWithoutPartialWrite(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)
	otherCPID := createTestCounterparty(t, h)
	receivableID := createTestTrade(t, h, cpID, "sale", 1000, "", nil)
	payableID := createTestTrade(t, h, cpID, "purchase", 800, "", nil)
	otherPayableID := createTestTrade(t, h, otherCPID, "purchase", 800, "", nil)

	for _, pair := range [][2]string{{receivableID, otherPayableID}, {receivableID, receivableID}} {
		rec := do(t, h, "POST", "/api/offsets", map[string]any{
			"receivable_trade_id": pair[0], "payable_trade_id": pair[1], "amount_cents": 100,
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid offset pair %v should return 400, got %d", pair, rec.Code)
		}
	}
	rec := do(t, h, "POST", "/api/offsets", map[string]any{
		"receivable_trade_id": receivableID, "payable_trade_id": payableID, "amount_cents": 801,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("offset beyond remaining amount should return 400, got %d", rec.Code)
	}
	rec = do(t, h, "GET", "/api/counterparties/"+cpID+"/timeline", nil)
	var timeline []service.TimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline) != 2 {
		t.Fatalf("rejected offsets should not write records, got %+v", timeline)
	}
}

func TestOffsetRejectsSameDirectionSettledOrMissingTrades(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)
	receivableID := createTestTrade(t, h, cpID, "sale", 1000, "", nil)
	payableID := createTestTrade(t, h, cpID, "purchase", 800, "", nil)
	sameDirectionID := createTestTrade(t, h, cpID, "service", 500, "", nil)

	for name, body := range map[string]map[string]any{
		"same direction": {
			"receivable_trade_id": receivableID,
			"payable_trade_id":    sameDirectionID,
			"amount_cents":        1,
		},
		"missing receivable": {
			"receivable_trade_id": "missing",
			"payable_trade_id":    payableID,
			"amount_cents":        1,
		},
		"missing payable": {
			"receivable_trade_id": receivableID,
			"payable_trade_id":    "missing",
			"amount_cents":        1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			rec := do(t, h, "POST", "/api/offsets", body)
			want := http.StatusBadRequest
			if strings.HasPrefix(name, "missing") {
				want = http.StatusNotFound
			}
			if rec.Code != want {
				t.Fatalf("got %d, want %d: %s", rec.Code, want, rec.Body.String())
			}
		})
	}

	if rec := do(t, h, "POST", "/api/trades/"+payableID+"/payments", map[string]any{"amount_cents": 800}); rec.Code != http.StatusCreated {
		t.Fatalf("settle payable trade: %d %s", rec.Code, rec.Body.String())
	}
	rec := do(t, h, "POST", "/api/offsets", map[string]any{
		"receivable_trade_id": receivableID,
		"payable_trade_id":    payableID,
		"amount_cents":        1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("settled payable trade should reject offset, got %d", rec.Code)
	}

	rec = do(t, h, "GET", "/api/counterparties/"+cpID+"/timeline", nil)
	var timeline []service.TimelineEntry
	if err := json.NewDecoder(rec.Body).Decode(&timeline); err != nil {
		t.Fatal(err)
	}
	for _, entry := range timeline {
		if entry.Kind == "offset" {
			t.Fatalf("rejected offsets should not write records, got %+v", timeline)
		}
	}
}

func TestTradeDirectionAndItemValidation(t *testing.T) {
	h := testServer(t)
	cpID := createTestCounterparty(t, h)

	for _, tc := range []struct {
		typ, direction string
	}{
		{typ: "sale", direction: "out"},
		{typ: "purchase", direction: "in"},
		{typ: "transfer", direction: ""},
	} {
		rec := do(t, h, "POST", "/api/trades", map[string]any{
			"counterparty_id": cpID,
			"type":            tc.typ,
			"direction":       tc.direction,
			"total_cents":     100,
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s + %q should be rejected, got %d", tc.typ, tc.direction, rec.Code)
		}
	}

	for _, item := range []service.TradeItem{{Name: "", Qty: 1, PriceCents: 10}, {Name: "网卡", Qty: 0, PriceCents: 10}, {Name: "网卡", Qty: 1, PriceCents: 0}} {
		rec := do(t, h, "POST", "/api/trades", map[string]any{
			"counterparty_id": cpID,
			"type":            "sale",
			"total_cents":     100,
			"items":           []service.TradeItem{item},
		})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("invalid item %+v should be rejected, got %d", item, rec.Code)
		}
	}

	tradeID := createTestTrade(t, h, cpID, "sale", 1000, "", []service.TradeItem{{Name: "网卡", Qty: 2, PriceCents: 300}, {Name: "服务", Qty: 1, PriceCents: 400}})
	rec := do(t, h, "GET", "/api/trades?counterparty_id="+cpID, nil)
	var trades []service.Trade
	if err := json.NewDecoder(rec.Body).Decode(&trades); err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 || trades[0].ID != tradeID || len(trades[0].Items) != 2 || trades[0].Items[0].Name != "网卡" {
		t.Fatalf("trade items should round trip, got %+v", trades)
	}
}

func TestAuthRequiresCaseSensitiveToken(t *testing.T) {
	t.Setenv("TRADE_LEDGER_PASSWORD", "Secret123")
	h := testServer(t)

	for _, tc := range []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "wrong case", header: "Authorization", value: "Bearer secret123", want: http.StatusUnauthorized},
		{name: "bearer", header: "Authorization", value: "Bearer Secret123", want: http.StatusOK},
		{name: "api token", header: "X-API-Token", value: "Secret123", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
	if rec := do(t, h, "GET", "/healthz", nil); rec.Code != http.StatusOK {
		t.Fatalf("healthz should remain public, got %d", rec.Code)
	}
}

func TestSPARoutesFallBackToIndex(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	webFS := http.FS(fstest.MapFS{
		"index.html":    &fstest.MapFile{Mode: fs.ModePerm, Data: []byte("<html>ledger</html>")},
		"assets/app.js": &fstest.MapFile{Mode: fs.ModePerm, Data: []byte("console.log('ledger')")},
	})
	h := New(service.New(db), webFS)

	for _, path := range []string{"/", "/home", "/parties", "/create", "/mine"} {
		rec := do(t, h, "GET", path, nil)
		if rec.Code != http.StatusOK || rec.Body.String() != "<html>ledger</html>" {
			t.Errorf("%s should serve index, got %d %q", path, rec.Code, rec.Body.String())
		}
	}
	rec := do(t, h, "GET", "/assets/missing.js", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing static resource should be 404, got %d", rec.Code)
	}
	rec = do(t, h, "GET", "/api/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown API resource should be 404, got %d", rec.Code)
	}
}
