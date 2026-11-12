package repo

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	// :memory: 每个连接独立，限制单连接保证并发测试共享同一份数据
	sqlDB.SetMaxOpenConns(1)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedProduct(t *testing.T, repo *GormRepository, stock int64) string {
	t.Helper()
	p, _ := entity.NewProduct("限量商品", 10000, stock)
	if err := repo.Save(context.Background(), p); err != nil {
		t.Fatalf("save product: %v", err)
	}
	return p.ID
}

// TestReserveConcurrentNoOversell：20 个并发争抢 5 件，成功占用必须恰好 5 次。
func TestReserveConcurrentNoOversell(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 5)

	var success atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			orderNo := fmt.Sprintf("ORD-CONC-%d", n)
			err := repo.Reserve(ctx, orderNo, productID, 1)
			if err == nil {
				success.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if success.Load() != 5 {
		t.Fatalf("并发占用成功次数 = %d, want 5（不允许超卖）", success.Load())
	}
	p, _ := repo.FindByID(ctx, productID)
	if p.Reserved != 5 || p.Available() != 0 {
		t.Fatalf("reserved = %d, available = %d, want 5/0", p.Reserved, p.Available())
	}
}

// TestReserveIdempotent：同订单重复占用不重复扣减。
func TestReserveIdempotent(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 5)
	if err := repo.Reserve(ctx, "ORD-1", productID, 2); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	if err := repo.Reserve(ctx, "ORD-1", productID, 2); err != nil {
		t.Fatalf("second reserve should be idempotent: %v", err)
	}
	p, _ := repo.FindByID(ctx, productID)
	if p.Reserved != 2 {
		t.Fatalf("reserved = %d, want 2", p.Reserved)
	}
}

// TestReserveOffSaleAndInsufficient：下架与库存不足时的失败路径。
func TestReserveOffSaleAndInsufficient(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 2)
	p, _ := repo.FindByID(ctx, productID)
	p.SetOnSale(false)
	_ = repo.Save(ctx, p)

	if err := repo.Reserve(ctx, "ORD-OFF", productID, 1); err != entity.ErrOffSale {
		t.Fatalf("下架商品下单应返回 ErrOffSale, got %v", err)
	}
	p.SetOnSale(true)
	_ = repo.Save(ctx, p)
	if err := repo.Reserve(ctx, "ORD-BIG", productID, 3); err != entity.ErrInsufficientStock {
		t.Fatalf("库存不足应返回 ErrInsufficientStock, got %v", err)
	}
}

// TestReleaseIdempotent：重复释放不多释放；未占用订单释放为空操作。
func TestReleaseIdempotent(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 5)
	_ = repo.Reserve(ctx, "ORD-R1", productID, 2)

	if err := repo.Release(ctx, "ORD-R1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := repo.Release(ctx, "ORD-R1"); err != nil {
		t.Fatalf("release twice: %v", err)
	}
	if err := repo.Release(ctx, "ORD-NONE"); err != nil {
		t.Fatalf("release unknown order: %v", err)
	}
	p, _ := repo.FindByID(ctx, productID)
	if p.Reserved != 0 || p.Available() != 5 {
		t.Fatalf("release 后 reserved = %d, available = %d, want 0/5", p.Reserved, p.Available())
	}
}

// TestConfirmAndRestore：支付确认扣减总量；退款恢复总量；均为幂等。
func TestConfirmAndRestore(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 5)
	_ = repo.Reserve(ctx, "ORD-C1", productID, 2)
	_ = repo.Confirm(ctx, "ORD-C1")
	_ = repo.Confirm(ctx, "ORD-C1") // 幂等
	p, _ := repo.FindByID(ctx, productID)
	if p.StockTotal != 3 || p.Reserved != 0 || p.Available() != 3 {
		t.Fatalf("confirm 后 stock = %d, reserved = %d, available = %d, want 3/0/3", p.StockTotal, p.Reserved, p.Available())
	}

	_ = repo.Restore(ctx, "ORD-C1")
	_ = repo.Restore(ctx, "ORD-C1") // 幂等
	p, _ = repo.FindByID(ctx, productID)
	if p.StockTotal != 5 || p.Available() != 5 {
		t.Fatalf("restore 后 stock = %d, available = %d, want 5/5", p.StockTotal, p.Available())
	}
}

// TestListIncludeOffSale：列表过滤在售状态。
func TestListIncludeOffSale(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	id := seedProduct(t, repo, 5)
	p, _ := repo.FindByID(ctx, id)
	p.SetOnSale(false)
	_ = repo.Save(ctx, p)

	all, _ := repo.List(ctx, true)
	onSale, _ := repo.List(ctx, false)
	if len(all) != 1 || len(onSale) != 0 {
		t.Fatalf("all = %d, onSale = %d, want 1/0", len(all), len(onSale))
	}
}
