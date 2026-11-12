package service

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"strings"
	"testing"
	"time"
)

var followupFailure = errors.New("followup storage unavailable")

type followupRepo struct {
	*fakeRepo
	findCalls, failFindAt                               int
	createFail, refundFail, rollbackFail, exceptionFail bool
}

func (r *followupRepo) FindByOrderNo(c context.Context, n string) (*entity.Order, error) {
	r.findCalls++
	if r.findCalls == r.failFindAt {
		return nil, followupFailure
	}
	return r.fakeRepo.FindByOrderNo(c, n)
}
func (r *followupRepo) Create(c context.Context, o *entity.Order) error {
	if r.createFail {
		return followupFailure
	}
	return r.fakeRepo.Create(c, o)
}
func (r *followupRepo) CreateRefundRequest(c context.Context, q *entity.RefundRequest) error {
	if r.refundFail {
		return followupFailure
	}
	return r.fakeRepo.CreateRefundRequest(c, q)
}
func (r *followupRepo) Transition(c context.Context, n, from, to string, u func(*entity.Order), at time.Time) (bool, error) {
	if r.rollbackFail && from == entity.StatusRefundRequested {
		return false, followupFailure
	}
	return r.fakeRepo.Transition(c, n, from, to, u, at)
}
func (r *followupRepo) CreatePayException(c context.Context, e *entity.PayException) error {
	if r.exceptionFail {
		return followupFailure
	}
	return r.fakeRepo.CreatePayException(c, e)
}

func TestFollowupCancelReloadFailures(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "timeout"}[expired], func(t *testing.T) {
			base := newFakeRepo()
			stock := &fakeStock{}
			seed := newTestService(base, &fakeChannel{}, stock)
			o := buyerCreate(t, seed, "buyer1")
			if expired {
				base.orders[o.OrderNo].PayExpireAt = time.Now().Add(-time.Minute)
			}
			r := &followupRepo{fakeRepo: base, failFindAt: 2}
			s := NewService(r, &fakeChannel{}, stock, time.Minute, "test-secret", nil)
			got, err := s.CancelOrder(ctx(), "buyer1", o.OrderNo, "cancel")
			if !errors.Is(err, followupFailure) || got != nil || stock.releases != 1 {
				t.Fatalf("reload failure must propagate after one release: got=%v err=%v releases=%d", got, err, stock.releases)
			}
		})
	}
}

func TestFollowupRefundCreationRollback(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "rollback", true: "rollback_failure"}[broken], func(t *testing.T) {
			base := newFakeRepo()
			seed := newTestService(base, &fakeChannel{}, &fakeStock{})
			o := buyerCreate(t, seed, "buyer1")
			if !payNotify(t, seed, o.OrderNo, "CH-ROLLBACK") {
				t.Fatal("setup")
			}
			r := &followupRepo{fakeRepo: base, refundFail: true, rollbackFail: broken}
			s := NewService(r, &fakeChannel{}, &fakeStock{}, time.Minute, "test-secret", nil)
			got, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: o.OrderNo, Reason: "return"})
			if got != nil || !errors.Is(err, followupFailure) {
				t.Fatalf("refund persistence failure lost: %v %v", got, err)
			}
			after, _ := base.FindByOrderNo(ctx(), o.OrderNo)
			want := entity.StatusPaid
			if broken {
				want = entity.StatusRefundRequested
				if !strings.Contains(err.Error(), "rollback") {
					t.Fatal(err)
				}
			}
			pending, _ := base.FindPendingRefundByOrder(ctx(), o.OrderNo)
			if after.Status != want || pending != nil {
				t.Fatalf("rollback status=%s want=%s pending=%v", after.Status, want, pending)
			}
		})
	}
}

func TestFollowupAuditWriteFailureIsNotAcknowledged(t *testing.T) {
	base := newFakeRepo()
	seed := newTestService(base, &fakeChannel{}, &fakeStock{})
	o := buyerCreate(t, seed, "buyer1")
	s := NewService(auditRecordWriteFailureRepo{base}, &fakeChannel{payTrade: "CH-ACCEPTED"}, &fakeStock{}, time.Minute, "test-secret", nil)
	if _, err := s.PayOrder(ctx(), "buyer1", o.OrderNo); err == nil || !strings.Contains(err.Error(), "CH-ACCEPTED") {
		t.Fatalf("accepted channel transaction audit error must be explicit: %v", err)
	}
	now := time.Now().Unix()
	req := NotifyInput{OrderNo: o.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-FAILED", Status: "FAILED", NotifiedAtUnix: now}
	req.Signature = sign(req.OrderNo, req.AmountCents, req.ChannelTradeNo, req.Status, now)
	if ok, reason := s.HandlePaymentNotify(ctx(), req); ok || !strings.Contains(reason, "record write failure") {
		t.Fatalf("failed audit acknowledged: %v %s", ok, reason)
	}
	changed, err := s.changeWithRecord(ctx(), &entity.OrderRecord{OrderNo: o.OrderNo}, func(entity.Repository) (bool, error) { return true, nil })
	if changed || err == nil {
		t.Fatalf("missing audit accepted: %v %v", changed, err)
	}
}

func TestFollowupAnomalyPersistenceFailure(t *testing.T) {
	r := &followupRepo{fakeRepo: newFakeRepo(), exceptionFail: true}
	s := NewService(r, &fakeChannel{}, &fakeStock{}, time.Minute, "test-secret", nil)
	if ok, reason := s.HandlePaymentNotify(ctx(), NotifyInput{OrderNo: "missing", ChannelTradeNo: "bad-signature"}); ok || !strings.Contains(reason, followupFailure.Error()) {
		t.Fatalf("anomaly failure swallowed: %v %s", ok, reason)
	}
	base := newFakeRepo()
	if err := base.CreatePayException(ctx(), &entity.PayException{OrderNo: "missing", ChannelTradeNo: "existing", Reason: "signature_invalid"}); err != nil {
		t.Fatal(err)
	}
	s = NewService(auditRecordWriteFailureRepo{base}, &fakeChannel{}, &fakeStock{}, time.Minute, "test-secret", nil)
	if ok, reason := s.HandlePaymentNotify(ctx(), NotifyInput{OrderNo: "missing", ChannelTradeNo: "existing"}); ok || !strings.Contains(reason, "record write failure") {
		t.Fatalf("raw duplicate audit failure swallowed: %v %s", ok, reason)
	}
}
