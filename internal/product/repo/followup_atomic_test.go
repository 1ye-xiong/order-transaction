package repo

import (
	"context"
	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"testing"
)

func TestFollowupAtomicUpdateRollsBackEveryColumnFailure(t *testing.T) {
	for _, column := range []string{"price_cents", "on_sale", "stock_total"} {
		t.Run(column, func(t *testing.T) {
			db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if e != nil {
				t.Fatal(e)
			}
			if e = AutoMigrate(db); e != nil {
				t.Fatal(e)
			}
			r := NewGormRepository(db, zap.NewNop())
			p, e := entity.NewProduct("atomic", 100, 5)
			if e != nil {
				t.Fatal(e)
			}
			c := context.Background()
			if e = r.Save(c, p); e != nil {
				t.Fatal(e)
			}
			if e = db.Exec("CREATE TRIGGER reject_update BEFORE UPDATE OF " + column + " ON products BEGIN SELECT RAISE(ABORT, 'injected write failure'); END").Error; e != nil {
				t.Fatal(e)
			}
			got, e := r.UpdateAtomic(c, dto.UpdateCommand{ID: p.ID, SetPrice: true, PriceCents: 200, SetOnSale: true, OnSale: false, StockDelta: 2})
			if e == nil || got != nil {
				t.Fatalf("write failure accepted %+v %v", got, e)
			}
			after, e := r.FindByID(c, p.ID)
			if e != nil || after.PriceCents != 100 || !after.OnSale || after.StockTotal != 5 {
				t.Fatalf("transaction failed rollback %+v %v", after, e)
			}
			if e = db.Exec("DROP TRIGGER reject_update").Error; e != nil {
				t.Fatal(e)
			}
			got, e = r.UpdateAtomic(c, dto.UpdateCommand{ID: p.ID, SetPrice: true, PriceCents: 200, SetOnSale: true, OnSale: false, StockDelta: 2})
			if e != nil || got.PriceCents != 200 || got.OnSale || got.StockTotal != 7 {
				t.Fatalf("retry %+v %v", got, e)
			}
			for _, cmd := range []dto.UpdateCommand{{ID: p.ID, SetPrice: true, PriceCents: 0}, {ID: p.ID, StockDelta: -1}, {ID: "missing"}} {
				if _, e = r.UpdateAtomic(c, cmd); e == nil {
					t.Fatalf("invalid update accepted %+v", cmd)
				}
			}
			bad, cancel := context.WithCancel(c)
			cancel()
			if _, e = r.UpdateAtomic(bad, dto.UpdateCommand{ID: p.ID}); e == nil {
				t.Fatal("cancelled transaction accepted")
			}
		})
	}
}
