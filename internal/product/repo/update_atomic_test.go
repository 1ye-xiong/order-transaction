package repo

import (
	"context"
	"testing"

	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUpdateAtomicDoesNotOverwriteReservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:update-atomic?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	r := NewGormRepository(db)
	p, err := entity.NewProduct("coffee", 10000, 5)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := r.Reserve(context.Background(), "ORD-ATOMIC-1", p.ID, 2); err != nil {
		t.Fatal(err)
	}
	updated, err := (&ServiceAdapter{r: r}).Update(context.Background(), dto.UpdateCommand{ID: p.ID, SetPrice: true, PriceCents: 9000})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Reserved != 2 || updated.StockTotal != 5 || updated.PriceCents != 9000 {
		t.Fatalf("stale update: %+v", updated)
	}
}

type ServiceAdapter struct{ r *GormRepository }

func (s *ServiceAdapter) Update(ctx context.Context, cmd dto.UpdateCommand) (*entity.Product, error) {
	return s.r.UpdateAtomic(ctx, cmd)
}
