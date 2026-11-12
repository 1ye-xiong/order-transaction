package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/product/repo"
)

type fallbackRepository struct{ entity.Repository }

func TestFallbackUpdateValidatesAndPersists(t *testing.T) {
	ctx := context.Background()
	_, r := newTestService(t)
	s := NewService(fallbackRepository{r})
	p, e := s.Create(ctx, dto.CreateCommand{Name: "fallback", PriceCents: 100, StockTotal: 2})
	if e != nil {
		t.Fatal(e)
	}
	for _, cmd := range []dto.UpdateCommand{{ID: "missing"}, {ID: p.ID, SetPrice: true, PriceCents: -1}, {ID: p.ID, StockDelta: -1}} {
		if _, e := s.Update(ctx, cmd); e == nil {
			t.Fatalf("invalid update accepted %+v", cmd)
		}
	}
	got, e := s.Update(ctx, dto.UpdateCommand{ID: p.ID, SetPrice: true, PriceCents: 200, SetOnSale: true, OnSale: false, StockDelta: 3})
	if e != nil || got.PriceCents != 200 || got.OnSale || got.StockTotal != 5 {
		t.Fatalf("update %+v %v", got, e)
	}
}

func newTestService(t *testing.T) (*Service, *repo.GormRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := repo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.NewGormRepository(db)
	return NewService(r), r
}

func TestCreateUpdateListAndGet(t *testing.T) {
	ctx := context.Background()
	s, r := newTestService(t)

	created, err := s.Create(ctx, dto.CreateCommand{Name: "蓝牙耳机", PriceCents: 10000, StockTotal: 5})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == "" || !created.OnSale || created.Available != 5 {
		t.Fatalf("created = %+v", created)
	}

	// 校验失败
	if _, err := s.Create(ctx, dto.CreateCommand{Name: "", PriceCents: 100, StockTotal: 1}); err == nil {
		t.Fatal("非法输入应失败")
	}

	// 调价 + 补货 + 下架
	updated, err := s.Update(ctx, dto.UpdateCommand{ID: created.ID, SetPrice: true, PriceCents: 9000, StockDelta: 3})
	if err != nil || updated.PriceCents != 9000 || updated.StockTotal != 8 {
		t.Fatalf("update: %v / %+v", err, updated)
	}
	updated, err = s.Update(ctx, dto.UpdateCommand{ID: created.ID, SetOnSale: true, OnSale: false})
	if err != nil || updated.OnSale {
		t.Fatalf("off sale: %v / %+v", err, updated)
	}
	if _, err := s.Update(ctx, dto.UpdateCommand{ID: "P-NONE", SetPrice: true, PriceCents: 1}); err == nil {
		t.Fatal("未知商品更新应失败")
	}
	if _, err := s.Update(ctx, dto.UpdateCommand{ID: created.ID, SetPrice: true, PriceCents: -1}); err == nil {
		t.Fatal("负价格应失败")
	}

	// 列表：默认不包含已下架
	all, _ := s.List(ctx, true)
	onSale, _ := s.List(ctx, false)
	if len(all) != 1 || len(onSale) != 0 {
		t.Fatalf("all = %d, onSale = %d", len(all), len(onSale))
	}

	// GetProduct
	got, err := s.GetProduct(ctx, created.ID)
	if err != nil || got.Name != "蓝牙耳机" {
		t.Fatalf("get: %v / %+v", err, got)
	}
	if _, err := s.GetProduct(ctx, "P-NONE"); err == nil {
		t.Fatal("未知商品查询应失败")
	}

	// 占用/释放/确认/恢复透传
	_, _ = s.Update(ctx, dto.UpdateCommand{ID: created.ID, SetOnSale: true, OnSale: true})
	if err := s.Reserve(ctx, "ORD-1", created.ID, 2); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := s.Reserve(ctx, "ORD-1", created.ID, 2); err != nil {
		t.Fatalf("reserve 幂等: %v", err)
	}
	res, err := r.FindReservation(ctx, "ORD-1")
	if err != nil || res.Quantity != 2 || res.Status != "reserved" {
		t.Fatalf("reservation = %+v, err = %v", res, err)
	}
	if _, err := r.FindReservation(ctx, "ORD-NONE"); err == nil {
		t.Fatal("未知订单占用应报错")
	}
	if err := s.Confirm(ctx, "ORD-1"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := s.Restore(ctx, "ORD-1"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	p, _ := s.GetProduct(ctx, created.ID)
	if p.StockTotal != 8 || p.Available != 8 {
		t.Fatalf("确认+恢复后库存 = %+v", p)
	}
	if err := s.Release(ctx, "ORD-NONE"); err != nil {
		t.Fatalf("释放未知订单应幂等成功: %v", err)
	}
}

func TestUpdateAtomicRejectsInvalidCommands(t *testing.T) {
	ctx := context.Background()
	s, r := newTestService(t)
	p, err := s.Create(ctx, dto.CreateCommand{Name: "atomic", PriceCents: 100, StockTotal: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []dto.UpdateCommand{{ID: p.ID, SetPrice: true, PriceCents: 0}, {ID: p.ID, StockDelta: -1}, {ID: "missing", SetOnSale: true, OnSale: false}} {
		if _, err := s.Update(ctx, cmd); err == nil {
			t.Fatalf("invalid atomic update accepted: %+v", cmd)
		}
	}
	if _, err := r.UpdateAtomic(ctx, dto.UpdateCommand{ID: p.ID, SetPrice: true, PriceCents: 200, SetOnSale: true, OnSale: false, StockDelta: 2}); err != nil {
		t.Fatal(err)
	}
}
