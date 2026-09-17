package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	db "github.com/EziosWJ/trade-ledger/internal/db"
	"github.com/google/uuid"
)

var (
	ErrOverpay       = errors.New("payment exceeds remaining amount")
	ErrBadRequest    = errors.New("bad request")
	ErrNotFound      = errors.New("not found")
	validTradeTypes  = map[string]bool{"sale": true, "service": true, "purchase": true, "transfer": true}
	validTags        = map[string]bool{"customer": true, "supplier": true, "peer": true, "individual": true}
	validDirections  = map[string]bool{"in": true, "out": true}
	defaultDirection = map[string]string{"sale": "in", "service": "in", "purchase": "out"}
)

type Counterparty struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Phone           string   `json:"phone"`
	Note            string   `json:"note"`
	Tags            []string `json:"tags"`
	CreatedAt       string   `json:"created_at"`
	ReceivableCents int64    `json:"receivable_cents"`
	PayableCents    int64    `json:"payable_cents"`
}

type Trade struct {
	ID             string      `json:"id"`
	CounterpartyID string      `json:"counterparty_id"`
	Type           string      `json:"type"`
	Direction      string      `json:"direction"`
	TotalCents     int64       `json:"total_cents"`
	PaidCents      int64       `json:"paid_cents"`
	RemainingCents int64       `json:"remaining_cents"`
	Note           string      `json:"note"`
	CreatedAt      string      `json:"created_at"`
	Items          []TradeItem `json:"items"`
}

type Offset struct {
	ID                       string `json:"id"`
	CounterpartyID           string `json:"counterparty_id"`
	ReceivableTradeID        string `json:"receivable_trade_id"`
	PayableTradeID           string `json:"payable_trade_id"`
	AmountCents              int64  `json:"amount_cents"`
	Note                     string `json:"note"`
	OccurredAt               string `json:"occurred_at"`
	ReceivableRemainingCents int64  `json:"receivable_remaining_cents"`
	PayableRemainingCents    int64  `json:"payable_remaining_cents"`
}

type TradeItem struct {
	Name       string `json:"name"`
	Qty        int    `json:"qty"`
	PriceCents int64  `json:"price_cents"`
}

type TimelineEntry struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	TradeID     string `json:"trade_id,omitempty"`
	OffsetID    string `json:"offset_id,omitempty"`
	AmountCents int64  `json:"amount_cents"`
	Note        string `json:"note"`
	At          string `json:"at"`
}

type Service struct {
	db *sql.DB
	q  *db.Queries
}

func New(database *sql.DB) *Service { return &Service{db: database, q: db.New(database)} }

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func joinTags(tags []string) (string, error) {
	for _, t := range tags {
		if !validTags[t] {
			return "", ErrBadRequest
		}
	}
	return strings.Join(tags, ","), nil
}

func splitTags(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func validateTradeItems(items []TradeItem) error {
	for i := range items {
		items[i].Name = strings.TrimSpace(items[i].Name)
		if items[i].Name == "" || items[i].Qty <= 0 || items[i].PriceCents <= 0 {
			return ErrBadRequest
		}
	}
	return nil
}

func (s *Service) CreateCounterparty(name, phone, note string, tags []string) (Counterparty, error) {
	if strings.TrimSpace(name) == "" {
		return Counterparty{}, ErrBadRequest
	}
	ts, err := joinTags(tags)
	if err != nil {
		return Counterparty{}, err
	}
	c := Counterparty{ID: uuid.NewString(), Name: name, Phone: phone, Note: note, Tags: tags, CreatedAt: now()}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	err = s.q.CreateCounterparty(context.Background(), db.CreateCounterpartyParams{
		ID: c.ID, Name: c.Name, Phone: c.Phone, Note: c.Note, Tags: ts, CreatedAt: c.CreatedAt,
	})
	return c, err
}

func (s *Service) ListCounterparties() ([]Counterparty, error) {
	balances := make(map[string][2]int64)
	balanceRows, err := s.q.ListCounterpartyBalances(context.Background())
	if err != nil {
		return nil, err
	}
	for _, row := range balanceRows {
		balances[row.CounterpartyID] = [2]int64{row.ReceivableCents, row.PayableCents}
	}

	rows, err := s.q.ListCounterparties(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]Counterparty, 0, len(rows))
	for _, row := range rows {
		c := Counterparty{ID: row.ID, Name: row.Name, Phone: row.Phone, Note: row.Note, CreatedAt: row.CreatedAt}
		c.Tags = splitTags(row.Tags)
		if balance, ok := balances[c.ID]; ok {
			c.ReceivableCents = balance[0]
			c.PayableCents = balance[1]
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Service) CreateTrade(counterpartyID, typ, direction string, totalCents int64, note string, items []TradeItem) (Trade, error) {
	if !validTradeTypes[typ] || totalCents <= 0 {
		return Trade{}, ErrBadRequest
	}
	if typ == "transfer" {
		if !validDirections[direction] {
			return Trade{}, ErrBadRequest
		}
	} else {
		expectedDirection := defaultDirection[typ]
		if direction == "" {
			direction = expectedDirection
		}
		if direction != expectedDirection {
			return Trade{}, ErrBadRequest
		}
	}
	if err := validateTradeItems(items); err != nil {
		return Trade{}, err
	}
	exists, err := s.q.CounterpartyExists(context.Background(), counterpartyID)
	if err != nil {
		return Trade{}, err
	}
	if exists == 0 {
		return Trade{}, ErrNotFound
	}
	if items == nil {
		items = []TradeItem{}
	}
	t := Trade{ID: uuid.NewString(), CounterpartyID: counterpartyID, Type: typ, Direction: direction,
		TotalCents: totalCents, Note: note, CreatedAt: now(), Items: items}
	tx, err := s.db.Begin()
	if err != nil {
		return Trade{}, err
	}
	defer tx.Rollback()
	txq := s.q.WithTx(tx)
	if err := txq.CreateTrade(context.Background(), db.CreateTradeParams{
		ID: t.ID, CounterpartyID: t.CounterpartyID, Type: t.Type, Direction: t.Direction,
		TotalCents: t.TotalCents, Note: t.Note, CreatedAt: t.CreatedAt,
	}); err != nil {
		return Trade{}, err
	}
	for _, it := range items {
		if err := txq.CreateTradeItem(context.Background(), db.CreateTradeItemParams{
			ID: uuid.NewString(), TradeID: t.ID, Name: it.Name, Qty: int64(it.Qty), PriceCents: it.PriceCents,
		}); err != nil {
			return Trade{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Trade{}, err
	}
	t.RemainingCents = totalCents
	return t, nil
}

func (s *Service) loadTradeItems(tradeID string) ([]TradeItem, error) {
	rows, err := s.q.ListTradeItems(context.Background(), tradeID)
	if err != nil {
		return nil, err
	}
	items := make([]TradeItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, TradeItem{Name: row.Name, Qty: int(row.Qty), PriceCents: row.PriceCents})
	}
	return items, nil
}

func (s *Service) paidCents(tradeID string) (int64, error) {
	return s.q.SettlementTotalByTrade(context.Background(), tradeID)
}

func remainingCents(total, settled int64) int64 {
	if settled >= total {
		return 0
	}
	return total - settled
}

func (s *Service) ListTrades(counterpartyID string) ([]Trade, error) {
	var rows []db.Trade
	var err error
	if counterpartyID != "" {
		rows, err = s.q.ListTradesByCounterparty(context.Background(), counterpartyID)
	} else {
		rows, err = s.q.ListTrades(context.Background())
	}
	if err != nil {
		return nil, err
	}
	out := make([]Trade, 0, len(rows))
	for _, row := range rows {
		out = append(out, Trade{
			ID: row.ID, CounterpartyID: row.CounterpartyID, Type: row.Type, Direction: row.Direction,
			TotalCents: row.TotalCents, Note: row.Note, CreatedAt: row.CreatedAt,
		})
	}
	for i := range out {
		paid, err := s.paidCents(out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].PaidCents = paid
		out[i].RemainingCents = remainingCents(out[i].TotalCents, paid)
		out[i].Items, err = s.loadTradeItems(out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) AddPayment(tradeID string, amountCents int64, note string) (int64, error) {
	if amountCents <= 0 {
		return 0, ErrBadRequest
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	txq := s.q.WithTx(tx)
	total, err := txq.GetTradeTotal(context.Background(), tradeID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	paid, err := txq.SettlementTotalByTrade(context.Background(), tradeID)
	if err != nil {
		return 0, err
	}
	if paid+amountCents > total {
		return 0, ErrOverpay
	}
	ts := now()
	if err := txq.CreatePayment(context.Background(), db.CreatePaymentParams{
		ID: uuid.NewString(), TradeID: tradeID, AmountCents: amountCents, Note: note, OccurredAt: ts, CreatedAt: ts,
	}); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return paid + amountCents, nil
}

func (s *Service) AddOffset(receivableTradeID, payableTradeID string, amountCents int64, note string) (Offset, error) {
	if amountCents <= 0 || receivableTradeID == "" || payableTradeID == "" || receivableTradeID == payableTradeID {
		return Offset{}, ErrBadRequest
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Offset{}, err
	}
	defer tx.Rollback()
	txq := s.q.WithTx(tx)
	ctx := context.Background()
	receivable, err := txq.GetTrade(ctx, receivableTradeID)
	if err == sql.ErrNoRows {
		return Offset{}, ErrNotFound
	}
	if err != nil {
		return Offset{}, err
	}
	payable, err := txq.GetTrade(ctx, payableTradeID)
	if err == sql.ErrNoRows {
		return Offset{}, ErrNotFound
	}
	if err != nil {
		return Offset{}, err
	}
	if receivable.CounterpartyID != payable.CounterpartyID || receivable.Direction != "in" || payable.Direction != "out" {
		return Offset{}, ErrBadRequest
	}
	receivableSettled, err := txq.SettlementTotalByTrade(ctx, receivableTradeID)
	if err != nil {
		return Offset{}, err
	}
	payableSettled, err := txq.SettlementTotalByTrade(ctx, payableTradeID)
	if err != nil {
		return Offset{}, err
	}
	receivableRemaining := remainingCents(receivable.TotalCents, receivableSettled)
	payableRemaining := remainingCents(payable.TotalCents, payableSettled)
	if amountCents > receivableRemaining || amountCents > payableRemaining {
		return Offset{}, ErrOverpay
	}

	offsetID := uuid.NewString()
	ts := now()
	if err := txq.CreateOffset(ctx, db.CreateOffsetParams{
		ID: offsetID, CounterpartyID: receivable.CounterpartyID, AmountCents: amountCents,
		Note: note, OccurredAt: ts, CreatedAt: ts,
	}); err != nil {
		return Offset{}, err
	}
	for _, tradeID := range []string{receivableTradeID, payableTradeID} {
		if err := txq.CreateOffsetAllocation(ctx, db.CreateOffsetAllocationParams{
			ID: uuid.NewString(), OffsetID: offsetID, TradeID: tradeID, AmountCents: amountCents, CreatedAt: ts,
		}); err != nil {
			return Offset{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Offset{}, err
	}
	return Offset{
		ID: offsetID, CounterpartyID: receivable.CounterpartyID,
		ReceivableTradeID: receivableTradeID, PayableTradeID: payableTradeID,
		AmountCents: amountCents, Note: note, OccurredAt: ts,
		ReceivableRemainingCents: receivableRemaining - amountCents,
		PayableRemainingCents:    payableRemaining - amountCents,
	}, nil
}

func (s *Service) Timeline(counterpartyID string) ([]TimelineEntry, error) {
	exists, err := s.q.CounterpartyExists(context.Background(), counterpartyID)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrNotFound
	}
	rows, err := s.q.ListTimelineEntries(context.Background(), counterpartyID)
	if err != nil {
		return nil, err
	}
	out := make([]TimelineEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, TimelineEntry{
			Kind: row.Kind, ID: row.ID, TradeID: row.TradeID,
			AmountCents: row.AmountCents, Note: row.Note, At: row.At, OffsetID: row.OffsetID,
		})
	}
	return out, nil
}

type Overview struct {
	ReceivableCents int64   `json:"receivable_cents"`
	PayableCents    int64   `json:"payable_cents"`
	Recent          []Trade `json:"recent"`
}

func (s *Service) Overview() (Overview, error) {
	balances, err := s.q.OverviewBalances(context.Background())
	if err != nil {
		return Overview{}, err
	}
	o := Overview{ReceivableCents: balances.ReceivableCents, PayableCents: balances.PayableCents}
	rows, err := s.q.ListRecentTrades(context.Background())
	if err != nil {
		return Overview{}, err
	}
	o.Recent = make([]Trade, 0, len(rows))
	for _, row := range rows {
		o.Recent = append(o.Recent, Trade{
			ID: row.ID, CounterpartyID: row.CounterpartyID, Type: row.Type, Direction: row.Direction,
			TotalCents: row.TotalCents, Note: row.Note, CreatedAt: row.CreatedAt,
		})
	}
	for i := range o.Recent {
		paid, err := s.paidCents(o.Recent[i].ID)
		if err != nil {
			return Overview{}, err
		}
		o.Recent[i].PaidCents = paid
		o.Recent[i].RemainingCents = remainingCents(o.Recent[i].TotalCents, paid)
		o.Recent[i].Items, err = s.loadTradeItems(o.Recent[i].ID)
		if err != nil {
			return Overview{}, err
		}
	}
	return o, nil
}
