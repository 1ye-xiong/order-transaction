package repo

import (
	"context"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuditShipmentRefundRacePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	// A real file database, multiple connections; busy contenders may retry through the API.
	sqlDB.SetMaxOpenConns(8)
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	r := NewGormRepository(db)
	for i := 0; i < 20; i++ {
		o := newOrder(time.Now().Format("150405.000000000"))
		o.Status = entity.StatusPaid
		if err := r.Create(context.Background(), o); err != nil {
			t.Fatal(err)
		}
		var wins atomic.Int64
		var wg sync.WaitGroup
		for _, target := range []string{entity.StatusShipped, entity.StatusRefundRequested} {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				changed, err := r.Transition(context.Background(), o.OrderNo, entity.StatusPaid, target, nil, time.Now())
				if err == nil && changed {
					wins.Add(1)
				}
			}(target)
		}
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("exactly one legal transition required, got %d", wins.Load())
		}
		got, err := r.FindByOrderNo(context.Background(), o.OrderNo)
		if err != nil || (got.Status != entity.StatusShipped && got.Status != entity.StatusRefundRequested) {
			t.Fatalf("persisted result: %+v %v", got, err)
		}
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	reopenedSQL, _ := reopened.DB()
	defer reopenedSQL.Close()
	items, err := NewGormRepository(reopened).ListByStatus(context.Background(), entity.StatusPaid)
	if err != nil || len(items) != 0 {
		t.Fatalf("restart left paid orders: %d %v", len(items), err)
	}
}

func TestAuditSummaryIgnoresUnverifiedExceptions(t *testing.T) {
	r := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	for _, ex := range []*entity.PayException{
		{OrderNo: "closed", ChannelTradeNo: "late", AmountCents: 10000, Reason: "late_payment", Status: entity.ExceptionPending},
		{OrderNo: "unknown", ChannelTradeNo: "bad", AmountCents: 99999, Reason: "signature_invalid", Status: entity.ExceptionRefunded},
	} {
		if err := r.CreatePayException(ctx, ex); err != nil {
			t.Fatal(err)
		}
	}
	paid, refunded, _, _, err := r.Summary(ctx)
	if err != nil || paid != 10000 || refunded != 0 {
		t.Fatalf("paid/refunded = %d/%d err=%v", paid, refunded, err)
	}
}
