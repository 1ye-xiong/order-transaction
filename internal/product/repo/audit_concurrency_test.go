package repo

import (
	"context"
	"fmt"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAuditConcurrentStockSettlementPersists(t *testing.T) {
	for _, action := range []string{"release", "confirm", "restore"} {
		t.Run(action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stock.db")
			db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(8)
			if err := AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			r := NewGormRepository(db)
			ctx := context.Background()
			id := seedProduct(t, r, 5)
			if err := r.Reserve(ctx, "same-order", id, 2); err != nil {
				t.Fatal(err)
			}
			fn := r.Release
			wantTotal := int64(5)
			if action == "confirm" {
				fn = r.Confirm
				wantTotal = 3
			}
			if action == "restore" {
				if err := r.Confirm(ctx, "same-order"); err != nil {
					t.Fatal(err)
				}
				fn = r.Restore
			}
			var wg sync.WaitGroup
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); _ = fn(ctx, "same-order") }()
			}
			wg.Wait()
			// Network/database contention is retryable; every repeated successful operation must be a no-op.
			if err := fn(ctx, "same-order"); err != nil {
				t.Fatal(err)
			}
			if err := sqlDB.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			reopened, _ := db.DB()
			defer reopened.Close()
			p, err := NewGormRepository(db).FindByID(ctx, id)
			if err != nil || p.StockTotal != wantTotal || p.Reserved != 0 {
				t.Fatalf("after restart stock=%+v err=%v", p, err)
			}
		})
	}
}

func TestAuditStockSettlementRollsBackOnProductWriteFailure(t *testing.T) {
	for _, action := range []string{"release", "confirm", "restore"} {
		t.Run(action, func(t *testing.T) {
			db := newTestDB(t)
			r := NewGormRepository(db)
			ctx := context.Background()
			id := seedProduct(t, r, 5)
			if err := r.Reserve(ctx, "rollback", id, 2); err != nil {
				t.Fatal(err)
			}
			fn := r.Release
			wantStatus := "reserved"
			if action == "confirm" {
				fn = r.Confirm
			}
			if action == "restore" {
				if err := r.Confirm(ctx, "rollback"); err != nil {
					t.Fatal(err)
				}
				fn = r.Restore
				wantStatus = "confirmed"
			}
			if err := db.Exec("DROP TABLE products").Error; err != nil {
				t.Fatal(err)
			}
			if err := fn(ctx, "rollback"); err == nil {
				t.Fatal("product write failure must be returned")
			}
			res, err := r.FindReservation(ctx, "rollback")
			if err != nil || res.Status != wantStatus {
				t.Fatalf("claim was not rolled back: %+v %v", res, err)
			}
		})
	}
}

func TestAuditStockSettlementClosedDatabase(t *testing.T) {
	db := newTestDB(t)
	r := NewGormRepository(db)
	conn, _ := db.DB()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func(context.Context, string) error{r.Release, r.Confirm, r.Restore} {
		if err := fn(context.Background(), "unavailable"); err == nil {
			t.Fatal("database error must propagate")
		}
	}
}

func TestAuditReservationClaimFailure(t *testing.T) {
	for _, action := range []string{"release", "confirm", "restore"} {
		t.Run(action, func(t *testing.T) {
			db := newTestDB(t)
			r := NewGormRepository(db)
			ctx := context.Background()
			id := seedProduct(t, r, 5)
			if err := r.Reserve(ctx, "claim", id, 2); err != nil {
				t.Fatal(err)
			}
			fn := r.Release
			wantStatus := "reserved"
			if action == "confirm" {
				fn = r.Confirm
			}
			if action == "restore" {
				if err := r.Confirm(ctx, "claim"); err != nil {
					t.Fatal(err)
				}
				fn = r.Restore
				wantStatus = "confirmed"
			}
			if err := db.Exec("CREATE TRIGGER reject_claim BEFORE UPDATE ON product_reservations BEGIN SELECT RAISE(ABORT, 'write failure'); END").Error; err != nil {
				t.Fatal(err)
			}
			if err := fn(ctx, "claim"); err == nil {
				t.Fatal("claim write error must propagate")
			}
			res, err := r.FindReservation(ctx, "claim")
			if err != nil || res.Status != wantStatus {
				t.Fatalf("bad state after failed claim: %+v %v", res, err)
			}
		})
	}
}

func TestAuditTwentyConnectionBuyFive(t *testing.T) {
	for run := 0; run < 5; run++ {
		path := filepath.Join(t.TempDir(), "race.db") + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		conn, _ := db.DB()
		conn.SetMaxOpenConns(20)
		if err := AutoMigrate(db); err != nil {
			t.Fatal(err)
		}
		r := NewGormRepository(db)
		id := seedProduct(t, r, 5)
		var success, insufficient atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				err := r.Reserve(context.Background(), fmt.Sprintf("twenty-%d", n), id, 1)
				if err == nil {
					success.Add(1)
				} else if err == entity.ErrInsufficientStock {
					insufficient.Add(1)
				} else {
					t.Errorf("unexpected reserve error: %v", err)
				}
			}(i)
		}
		wg.Wait()
		if success.Load() != 5 || insufficient.Load() != 15 {
			t.Fatalf("want 5 success/15 insufficient, got %d/%d", success.Load(), insufficient.Load())
		}
		conn.Close()
	}
}
