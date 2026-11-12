package repo

import (
	"context"
	"testing"

	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
)

func TestFindReservation(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 10)
	if err := repo.Reserve(ctx, "O-R1", productID, 2); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	got, err := repo.FindReservation(ctx, "O-R1")
	if err != nil || got.ProductID != productID || got.Quantity != 2 || got.Status != entity.ReservationReserved {
		t.Fatalf("FindReservation: %v / %+v", err, got)
	}
	if _, err := repo.FindReservation(ctx, "O-NONE"); err != entity.ErrReservationMissing {
		t.Fatalf("未知占用应 ErrReservationMissing, got %v", err)
	}
}

func TestReservationStatusTransitions(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 10)
	if err := repo.Reserve(ctx, "O-R2", productID, 2); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// 已释放的占用不可确认；重复释放不重复减
	if err := repo.Release(ctx, "O-R2"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := repo.Confirm(ctx, "O-R2"); err != nil {
		t.Fatalf("确认已释放占用应幂等返回 nil, got %v", err)
	}
	if err := repo.Restore(ctx, "O-R2"); err != nil {
		t.Fatalf("恢复未确认占用应幂等返回 nil, got %v", err)
	}
	if err := repo.Release(ctx, "O-R2"); err != nil {
		t.Fatalf("重复释放应幂等: %v", err)
	}

	// 未知订单：Release/Restore 幂等返回 nil，Confirm 报 ErrReservationMissing
	if err := repo.Release(ctx, "O-NONE"); err != nil {
		t.Fatalf("未知订单释放应幂等: %v", err)
	}
	if err := repo.Restore(ctx, "O-NONE"); err != nil {
		t.Fatalf("未知订单恢复应幂等: %v", err)
	}
	if err := repo.Confirm(ctx, "O-NONE"); err != entity.ErrReservationMissing {
		t.Fatalf("未知订单确认应 ErrReservationMissing, got %v", err)
	}
}

func TestReserveInvalidQuantity(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	productID := seedProduct(t, repo, 5)
	if err := repo.Reserve(ctx, "O-BAD", productID, 0); err != entity.ErrInvalidQuantity {
		t.Fatalf("数量为 0 应拒绝, got %v", err)
	}
	if err := repo.Reserve(ctx, "O-BAD", "NOPE", 1); err != entity.ErrProductNotFound {
		t.Fatalf("未知商品应拒绝, got %v", err)
	}
}
