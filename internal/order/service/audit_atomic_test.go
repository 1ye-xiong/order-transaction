package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	orderrepo "github.com/BwCloudWeGo/bw-cli/internal/order/repo"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAuditAtomicRefundServiceWithDatabase(t *testing.T) {
	for _, approve := range []bool{false, true} {
		t.Run(map[bool]string{false: "reject", true: "approve"}[approve], func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := orderrepo.AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			r := orderrepo.NewGormRepository(db)
			channel := &fakeChannel{}
			stock := &fakeStock{}
			s := NewService(r, channel, stock, 10*time.Minute, "test-secret", nil)
			o := buyerCreate(t, s, "buyer1")
			if !payNotify(t, s, o.OrderNo, "CH-ATOMIC") {
				t.Fatal("payment failed")
			}
			applied, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: o.OrderNo, Reason: "不需要"})
			if err != nil || applied.Status != entity.StatusRefundRequested {
				t.Fatalf("atomic apply: %v / %+v", err, applied)
			}
			req, err := r.FindPendingRefundByOrder(ctx(), o.OrderNo)
			if err != nil || req == nil {
				t.Fatalf("application not committed: %v", err)
			}
			reviewed, err := s.ReviewRefund(ctx(), dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: req.ID, Approve: approve, Reason: "核实结果"})
			expected := entity.StatusPaid
			if approve {
				expected = entity.StatusRefunded
			}
			if err != nil || reviewed.Status != expected {
				t.Fatalf("atomic review: %v / %+v", err, reviewed)
			}
			saved, _ := r.FindRefundRequest(ctx(), req.ID)
			if saved.ReviewReason != "核实结果" || saved.ReviewerID != "cs01" {
				t.Fatalf("review audit missing: %+v", saved)
			}
		})
	}
}

type auditAtomicRepo struct {
	*fakeRepo
	changed  bool
	err      error
	afterErr bool
}

func (r *auditAtomicRepo) ApplyRefundAtomic(context.Context, string, string, string, time.Time) (bool, error) {
	if r.afterErr {
		r.fakeRepo.mu.Lock()
		delete(r.fakeRepo.orders, "ORD-ATOMIC")
		r.fakeRepo.mu.Unlock()
	}
	return r.changed, r.err
}
func (r *auditAtomicRepo) ReviewRefundAtomic(context.Context, *entity.RefundRequest, bool, time.Time) (bool, error) {
	if r.afterErr {
		r.fakeRepo.mu.Lock()
		delete(r.fakeRepo.orders, "ORD-ATOMIC")
		r.fakeRepo.mu.Unlock()
	}
	return r.changed, r.err
}

func TestAuditAtomicRefundFailureNeverCallsChannel(t *testing.T) {
	for _, mode := range []string{"database failure", "lost state race", "reload failure"} {
		for _, review := range []bool{false, true} {
			t.Run(mode+map[bool]string{false: " apply", true: " review"}[review], func(t *testing.T) {
				r := &auditAtomicRepo{fakeRepo: newFakeRepo(), changed: mode != "lost state race", afterErr: mode == "reload failure"}
				if mode == "database failure" {
					r.err = errors.New("injected atomic write failure")
				}
				status := entity.StatusPaid
				if review {
					status = entity.StatusRefundRequested
				}
				_ = r.Create(ctx(), &entity.Order{OrderNo: "ORD-ATOMIC", BuyerID: "buyer1", Status: status, PayExpireAt: time.Now().Add(time.Minute)})
				channel := &fakeChannel{}
				s := NewService(r, channel, &fakeStock{}, time.Minute, "test-secret", nil)
				var err error
				if review {
					req := &entity.RefundRequest{OrderNo: "ORD-ATOMIC", BuyerID: "buyer1", Status: entity.RefundPending}
					_ = r.CreateRefundRequest(ctx(), req)
					_, err = s.ReviewRefund(ctx(), dto.ReviewRefundCommand{RefundID: req.ID, ReviewerID: "cs01", Reason: "核实拒绝", Approve: false})
				} else {
					_, err = s.ApplyRefund(ctx(), dto.ApplyRefundCommand{OrderNo: "ORD-ATOMIC", BuyerID: "buyer1", Reason: "不需要"})
				}
				if err == nil || len(channel.refunds) != 0 {
					t.Fatalf("failed atomic operation must not refund: err=%v refunds=%v", err, channel.refunds)
				}
			})
		}
	}
}
