package repo

import (
	"context"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"testing"
	"time"
)

func TestAuditAtomicRefundApplicationAndReview(t *testing.T) {
	for _, approve := range []bool{true, false} {
		t.Run(map[bool]string{true: "approve", false: "reject"}[approve], func(t *testing.T) {
			r := NewGormRepository(newTestDB(t))
			ctx := context.Background()
			now := time.Now()
			o := newOrder("atomic")
			o.Status = entity.StatusPaid
			if err := r.Create(ctx, o); err != nil {
				t.Fatal(err)
			}
			if ok, err := r.ApplyRefundAtomic(ctx, o.OrderNo, "other", "reason", now); err != nil || ok {
				t.Fatalf("ownership %v %v", ok, err)
			}
			if ok, err := r.ApplyRefundAtomic(ctx, o.OrderNo, o.BuyerID, "reason", now); err != nil || !ok {
				t.Fatalf("apply %v %v", ok, err)
			}
			if ok, err := r.ApplyRefundAtomic(ctx, o.OrderNo, o.BuyerID, "duplicate", now); err != nil || ok {
				t.Fatalf("duplicate %v %v", ok, err)
			}
			req, err := r.FindPendingRefundByOrder(ctx, o.OrderNo)
			if err != nil || req == nil {
				t.Fatal(err)
			}
			req.ReviewerID = "staff"
			req.ReviewReason = "verified"
			if ok, err := r.ReviewRefundAtomic(ctx, req, approve, now); err != nil || !ok {
				t.Fatalf("review %v %v", ok, err)
			}
			if ok, err := r.ReviewRefundAtomic(ctx, req, approve, now); err != nil || ok {
				t.Fatalf("duplicate review %v %v", ok, err)
			}
			got, _ := r.FindByOrderNo(ctx, o.OrderNo)
			want := entity.StatusPaid
			if approve {
				want = entity.StatusRefundRequested
			}
			if got.Status != want {
				t.Fatalf("state %s want %s", got.Status, want)
			}
		})
	}
}

func TestAuditAtomicRefundRollback(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	t.Run("application insert error", func(t *testing.T) {
		r := NewGormRepository(newTestDB(t))
		o := newOrder("rollback")
		o.Status = entity.StatusPaid
		if err := r.Create(ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := r.db.Exec("DROP TABLE refund_requests").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := r.ApplyRefundAtomic(ctx, o.OrderNo, o.BuyerID, "reason", now); err == nil {
			t.Fatal("expected insert failure")
		}
		got, _ := r.FindByOrderNo(ctx, o.OrderNo)
		if got.Status != entity.StatusPaid {
			t.Fatal("state must roll back")
		}
	})
	t.Run("review wrong state", func(t *testing.T) {
		r := NewGormRepository(newTestDB(t))
		o := newOrder("wrong")
		if err := r.Create(ctx, o); err != nil {
			t.Fatal(err)
		}
		req := &entity.RefundRequest{OrderNo: o.OrderNo, Status: entity.RefundPending}
		if err := r.CreateRefundRequest(ctx, req); err != nil {
			t.Fatal(err)
		}
		if _, err := r.ReviewRefundAtomic(ctx, req, false, now); err != entity.ErrInvalidState {
			t.Fatalf("expected invalid state, %v", err)
		}
		got, _ := r.FindRefundRequest(ctx, req.ID)
		if got.Status != entity.RefundPending {
			t.Fatal("review must roll back")
		}
	})
	t.Run("closed database", func(t *testing.T) {
		r := NewGormRepository(newTestDB(t))
		db, _ := r.db.DB()
		db.Close()
		if _, err := r.ApplyRefundAtomic(ctx, "o", "b", "reason", now); err == nil {
			t.Fatal("missing database error")
		}
		if _, err := r.ReviewRefundAtomic(ctx, &entity.RefundRequest{}, true, now); err == nil {
			t.Fatal("missing database error")
		}
		if _, err := r.MarkPaid(ctx, "o", "t", now); err == nil {
			t.Fatal("missing database error")
		}
		if err := r.CreatePayException(ctx, &entity.PayException{}); err == nil {
			t.Fatal("missing database error")
		}
	})
}

func TestAuditCrossTablePaymentFacts(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	r := NewGormRepository(newTestDB(t))
	for _, no := range []string{"one", "two", "three"} {
		if err := r.Create(ctx, newOrder(no)); err != nil {
			t.Fatal(err)
		}
	}
	ex := &entity.PayException{OrderNo: "closed", ChannelTradeNo: "claimed-late", Reason: "late_payment", AmountCents: 100, Status: entity.ExceptionPending}
	if err := r.CreatePayException(ctx, ex); err != nil {
		t.Fatal(err)
	}
	if _, err := r.MarkPaid(ctx, "one", "claimed-late", now); err != entity.ErrTradeNoConflict {
		t.Fatalf("late conflict %v", err)
	}
	if err := r.CreatePayException(ctx, &entity.PayException{OrderNo: "other", ChannelTradeNo: "claimed-late", Reason: "late_payment", AmountCents: 100}); err != entity.ErrTradeNoConflict {
		t.Fatalf("exception conflict %v", err)
	}
	if err := r.CreatePayException(ctx, &entity.PayException{OrderNo: "closed", ChannelTradeNo: "claimed-late", Reason: "late_payment", AmountCents: 100}); err != nil {
		t.Fatalf("duplicate %v", err)
	}
	if ok, err := r.MarkPaid(ctx, "two", "paid-trade", now); err != nil || !ok {
		t.Fatalf("paid %v %v", ok, err)
	}
	if err := r.CreatePayException(ctx, &entity.PayException{OrderNo: "other", ChannelTradeNo: "paid-trade", Reason: "late_payment", AmountCents: 100}); err != entity.ErrTradeNoConflict {
		t.Fatalf("paid conflict %v", err)
	}
	invalid := &entity.PayException{OrderNo: "three", ChannelTradeNo: "invalid", Reason: "signature_invalid", Detail: "original forensic event", AmountCents: 100}
	if err := r.CreatePayException(ctx, invalid); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.MarkPaid(ctx, "three", "invalid", now); err != nil || !ok {
		t.Fatalf("invalid must not block payment %v %v", ok, err)
	}
	correction := &entity.PayException{OrderNo: "closed-other", ChannelTradeNo: "corrected", Reason: "signature_invalid", Detail: "original", AmountCents: 100}
	if err := r.CreatePayException(ctx, correction); err != nil {
		t.Fatal(err)
	}
	correction.Reason = "late_payment"
	correction.Detail = "corrected"
	if err := r.CreatePayException(ctx, correction); err != nil {
		t.Fatal(err)
	}
	got, _ := r.FindPayException(ctx, correction.ID)
	if got.Reason != "late_payment" || got.Detail == "corrected" {
		t.Fatalf("must preserve old detail: %+v", got)
	}
}
