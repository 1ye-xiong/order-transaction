package repo

import (
	"context"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

func TestListExpiredPending(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-E1"))
	_ = repo.Create(ctx, newOrder("ORD-E2"))
	repo.db.Model(&dbmodel.OrderModel{}).Where("order_no = ?", "ORD-E1").
		Update("pay_expire_at", time.Now().UTC().Add(-time.Minute))

	expired, err := repo.ListExpiredPending(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListExpiredPending: %v", err)
	}
	if len(expired) != 1 || expired[0].OrderNo != "ORD-E1" {
		t.Fatalf("expired = %+v", expired)
	}
}

func TestTransitionApplyCallback(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-AP"))
	now := time.Now().UTC()
	applied := false
	ok, err := repo.Transition(ctx, "ORD-AP", entity.StatusPendingPayment, entity.StatusPaid, func(o *entity.Order) {
		applied = true
		o.ChannelTradeNo = "CH-AP"
		paid := now
		o.PaidAt = &paid
	}, now)
	if err != nil || !ok || !applied {
		t.Fatalf("transition: ok = %v, applied = %v, err = %v", ok, applied, err)
	}
	got, _ := repo.FindByOrderNo(ctx, "ORD-AP")
	if got.Status != entity.StatusPaid || got.PaidAt == nil {
		t.Fatalf("apply 未落库: %+v", got)
	}
}

func TestRefundRequestQueries(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-RQ"))

	if none, err := repo.FindPendingRefundByOrder(ctx, "ORD-RQ"); err != nil || none != nil {
		t.Fatalf("无申请应返回 nil: %v / %+v", err, none)
	}

	first := &entity.RefundRequest{OrderNo: "ORD-RQ", BuyerID: "buyer1", Reason: "a", Status: entity.RefundPending, CreatedAt: time.Now().UTC()}
	second := &entity.RefundRequest{OrderNo: "ORD-RQ", BuyerID: "buyer1", Reason: "b", Status: entity.RefundRejected, CreatedAt: time.Now().UTC()}
	if err := repo.CreateRefundRequest(ctx, first); err != nil {
		t.Fatalf("create 1: %v", err)
	}
	if err := repo.CreateRefundRequest(ctx, second); err != nil {
		t.Fatalf("create 2: %v", err)
	}

	pending, err := repo.ListRefundRequests(ctx, entity.RefundPending)
	if err != nil || len(pending) != 1 || pending[0].ID != first.ID {
		t.Fatalf("按状态过滤: %v / %+v", err, pending)
	}
	all, err := repo.ListRefundRequests(ctx, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("全量查询: %v / %d", err, len(all))
	}
	if all[0].ID != second.ID {
		t.Fatalf("应按 id 倒序: %+v", all)
	}
	latest, err := repo.FindLatestRefundByOrder(ctx, "ORD-RQ")
	if err != nil || latest == nil || latest.ID != second.ID {
		t.Fatalf("最新申请 = %+v, err = %v", latest, err)
	}
	if _, err := repo.FindRefundRequest(ctx, 9999); err == nil {
		t.Fatal("未知退款申请应报错")
	}
}

func TestPayExceptionByID(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	ex := &entity.PayException{OrderNo: "ORD-X", ChannelTradeNo: "CH-X", AmountCents: 5000, Reason: "late", Status: entity.ExceptionPending}
	if err := repo.CreatePayException(ctx, ex); err != nil {
		t.Fatalf("create exception: %v", err)
	}
	got, err := repo.FindPayException(ctx, ex.ID)
	if err != nil || got.ChannelTradeNo != "CH-X" || got.Status != entity.ExceptionPending {
		t.Fatalf("FindPayException: %v / %+v", err, got)
	}
	all, err := repo.ListPayExceptions(ctx, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("ListPayExceptions: %v / %d", err, len(all))
	}
	if _, err := repo.FindPayException(ctx, 9999); err == nil {
		t.Fatal("未知异常应报错")
	}
}

func TestSummary(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()

	paid := newOrder("ORD-S1")
	paid.Status = entity.StatusPaid
	paid.ChannelTradeNo = "CH-S1"
	_ = repo.Create(ctx, paid)

	shipped := newOrder("ORD-S2")
	shipped.Status = entity.StatusShipped
	shipped.ChannelTradeNo = "CH-S2"
	_ = repo.Create(ctx, shipped)

	refunded := newOrder("ORD-S3")
	refunded.Status = entity.StatusRefunded
	refunded.ChannelTradeNo = "CH-S3"
	_ = repo.Create(ctx, refunded)

	pending := newOrder("ORD-S4")
	_ = repo.Create(ctx, pending)

	late := &entity.PayException{OrderNo: "ORD-S1", ChannelTradeNo: "CH-LATE", AmountCents: 3000, Reason: "late_payment", Status: entity.ExceptionRefunded}
	_ = repo.CreatePayException(ctx, late)

	paidCents, refundedCents, counts, exceptions, err := repo.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if paidCents != 33000 {
		t.Fatalf("已收款 = %d, want 33000（含迟到支付3000）", paidCents)
	}
	if refundedCents != 13000 {
		t.Fatalf("已退款 = %d, want 13000（订单 10000 + 异常 3000）", refundedCents)
	}
	if counts[entity.StatusPendingPayment] != 1 || counts[entity.StatusPaid] != 1 ||
		counts[entity.StatusShipped] != 1 || counts[entity.StatusRefunded] != 1 {
		t.Fatalf("状态计数 = %+v", counts)
	}
	if exceptions != 1 {
		t.Fatalf("异常数 = %d", exceptions)
	}
}
