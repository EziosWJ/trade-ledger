package service

import (
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/EziosWJ/trade-ledger/internal/store"
	"github.com/EziosWJ/trade-ledger/migrations"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestConcurrentPaymentsCannotOverpay(t *testing.T) {
	svc := testService(t)
	cp, err := svc.CreateCounterparty("并发测试", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	trade, err := svc.CreateTrade(cp.ID, "sale", "", 1000, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.AddPayment(trade.ID, 600, "")
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if err != ErrOverpay {
			t.Fatalf("unexpected concurrent payment error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("exactly one payment should succeed, got %d", successes)
	}
	trades, err := svc.ListTrades(cp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 || trades[0].PaidCents != 600 || trades[0].RemainingCents != 400 {
		t.Fatalf("concurrent payments should leave 400 cents, got %+v", trades)
	}
}

func TestConcurrentOffsetAndPaymentCannotOverSettle(t *testing.T) {
	svc := testService(t)
	cp, err := svc.CreateCounterparty("抵扣并发测试", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	receivable, err := svc.CreateTrade(cp.ID, "sale", "", 1000, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	payable, err := svc.CreateTrade(cp.ID, "purchase", "", 1000, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := svc.AddOffset(receivable.ID, payable.ID, 600, "")
		results <- err
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := svc.AddPayment(receivable.ID, 600, "")
		results <- err
	}()
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if err != ErrOverpay {
			t.Fatalf("unexpected concurrent settlement error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("exactly one settlement should succeed, got %d", successes)
	}
	receivableTrades, err := svc.ListTrades(cp.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, trade := range receivableTrades {
		if trade.RemainingCents < 0 {
			t.Fatalf("concurrent settlements must not create negative balance: %+v", trade)
		}
	}
}
