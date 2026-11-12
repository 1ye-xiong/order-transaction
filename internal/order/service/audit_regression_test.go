package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// 独立回归：有效签名不等于付款成功，未知/处理中状态不得入账。
func TestAuditNonSuccessNotifyNeverMarksPaid(t *testing.T) {
	for _, status := range []string{"PENDING", "PROCESSING", "", "success", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			repo, stock, s, orderNo := setupPaidOrder(t)
			now := time.Now().Unix()
			in := NotifyInput{OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-PENDING", Status: status, NotifiedAtUnix: now}
			in.Signature = sign(orderNo, in.AmountCents, in.ChannelTradeNo, status, now)
			accepted, _ := s.HandlePaymentNotify(ctx(), in)
			if accepted {
				t.Fatal("non-success status must be rejected")
			}
			o, err := repo.FindByOrderNo(ctx(), orderNo)
			if err != nil || o.Status != entity.StatusPendingPayment || o.ChannelTradeNo != "" || stock.confirms != 0 {
				t.Fatalf("invalid notification mutated payment: order=%+v confirms=%d err=%v", o, stock.confirms, err)
			}
		})
	}
}

func TestAuditRefundReviewRequiresMeaningfulReason(t *testing.T) {
	for _, approve := range []bool{true, false} {
		for _, reason := range []string{"", " ", "\t\n", "　"} {
			t.Run(reason+map[bool]string{true: "approve", false: "reject"}[approve], func(t *testing.T) {
				repo := newFakeRepo()
				channel := &fakeChannel{}
				stock := &fakeStock{}
				s := newTestService(repo, channel, stock)
				order := buyerCreate(t, s, "buyer1")
				if !payNotify(t, s, order.OrderNo, "CH-REASON") {
					t.Fatal("payment setup failed")
				}
				if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不需要了"}); err != nil {
					t.Fatal(err)
				}
				req, err := repo.FindPendingRefundByOrder(ctx(), order.OrderNo)
				if err != nil || req == nil {
					t.Fatalf("request missing: %v", err)
				}
				_, err = s.ReviewRefund(ctx(), dto.ReviewRefundCommand{RefundID: req.ID, ReviewerID: "cs01", Approve: approve, Reason: reason})
				if !errors.Is(err, entity.ErrReasonRequired) {
					t.Fatalf("review with blank reason: got %v, want ErrReasonRequired", err)
				}
				after, _ := repo.FindRefundRequest(ctx(), req.ID)
				o, _ := repo.FindByOrderNo(ctx(), order.OrderNo)
				if after.Status != entity.RefundPending || o.Status != entity.StatusRefundRequested || len(channel.refunds) != 0 || stock.restores != 0 {
					t.Fatalf("rejected review must not mutate state or refund: request=%+v order=%+v refunds=%v restores=%d", after, o, channel.refunds, stock.restores)
				}
			})
		}
	}
}

type auditReleaseFailureStock struct {
	fakeStock
	calls    int
	released bool
}

func (s *auditReleaseFailureStock) Release(_ context.Context, _ string) error {
	s.calls++
	if s.calls == 1 {
		return errors.New("injected release transport failure")
	}
	if !s.released {
		s.released = true
		s.releases++
	}
	return nil
}

// 首次 RPC 失败后重新创建服务，防止仅靠进程内记忆补偿。
func TestAuditCancelRetriesReleaseAfterServiceRestart(t *testing.T) {
	repo := newFakeRepo()
	stock := &auditReleaseFailureStock{}
	s := newTestService(repo, &fakeChannel{}, stock)
	order := buyerCreate(t, s, "buyer1")
	if _, err := s.CancelOrder(ctx(), "buyer1", order.OrderNo, "不要了"); err == nil {
		t.Fatal("first cancel must report stock release failure")
	}
	s = newTestService(repo, &fakeChannel{}, stock)
	got, err := s.CancelOrder(ctx(), "buyer1", order.OrderNo, "不要了")
	if err != nil || got == nil || got.Status != entity.StatusCancelled || stock.releases != 1 {
		t.Fatalf("cancel retry did not recover release: order=%+v err=%v releaseCalls=%d released=%d", got, err, stock.calls, stock.releases)
	}
	if _, err := s.CancelOrder(ctx(), "buyer1", order.OrderNo, "不要了"); err != nil || stock.releases != 1 {
		t.Fatalf("repeat cancel must not release quantity twice: %v / %d", err, stock.releases)
	}
}

type auditRestoreFailureStock struct {
	fakeStock
	calls int
}

func (s *auditRestoreFailureStock) Restore(context.Context, string) error {
	s.calls++
	if s.calls == 1 {
		return errors.New("injected restore transport failure")
	}
	s.restores++
	return nil
}

func TestAuditRefundRestoreRecoversWithoutSecondChannelRefund(t *testing.T) {
	repo := newFakeRepo()
	channel := &fakeChannel{}
	stock := &auditRestoreFailureStock{}
	s := newTestService(repo, channel, stock)
	order := buyerCreate(t, s, "buyer1")
	if !payNotify(t, s, order.OrderNo, "CH-RESTORE") {
		t.Fatal("payment setup failed")
	}
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不需要"}); err != nil {
		t.Fatal(err)
	}
	req, _ := repo.FindPendingRefundByOrder(ctx(), order.OrderNo)
	if _, err := s.ReviewRefund(ctx(), dto.ReviewRefundCommand{RefundID: req.ID, ReviewerID: "cs01", Approve: true, Reason: "核实同意"}); err == nil {
		t.Fatal("restore failure must be reported")
	}
	s = newTestService(repo, channel, stock)
	after, err := s.RetryRefund(ctx(), "cs01", order.OrderNo)
	if err != nil || after == nil || after.Status != entity.StatusRefunded || stock.restores != 1 || len(channel.refunds) != 1 {
		t.Fatalf("stock compensation must not refund money twice: order=%+v err=%v restored=%d channel=%v", after, err, stock.restores, channel.refunds)
	}
}

type auditConfirmFailureStock struct {
	fakeStock
	calls int
}

func (s *auditConfirmFailureStock) Confirm(context.Context, string) error {
	s.calls++
	if s.calls == 1 {
		return errors.New("injected confirm transport failure")
	}
	s.confirms++
	return nil
}

func TestAuditDuplicatePaymentRecoversStockConfirmation(t *testing.T) {
	repo := newFakeRepo()
	stock := &auditConfirmFailureStock{}
	s := newTestService(repo, &fakeChannel{}, stock)
	order := buyerCreate(t, s, "buyer1")
	now := time.Now().Unix()
	in := NotifyInput{OrderNo: order.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-RECOVER-CONFIRM", Status: "SUCCESS", NotifiedAtUnix: now}
	in.Signature = sign(in.OrderNo, in.AmountCents, in.ChannelTradeNo, in.Status, now)
	if accepted, _ := s.HandlePaymentNotify(ctx(), in); accepted {
		t.Fatal("confirm failure must not be acknowledged")
	}
	s = newTestService(repo, &fakeChannel{}, stock)
	if accepted, reason := s.HandlePaymentNotify(ctx(), in); !accepted || stock.confirms != 1 {
		t.Fatalf("duplicate notify must recover confirm: accepted=%v reason=%s confirmed=%d", accepted, reason, stock.confirms)
	}
	if accepted, reason := s.HandlePaymentNotify(ctx(), in); !accepted || stock.confirms != 1 {
		t.Fatalf("third notify must not duplicate confirm: accepted=%v reason=%s confirmed=%d", accepted, reason, stock.confirms)
	}
}

type auditRecordReadFailureRepo struct{ *fakeRepo }

func (r auditRecordReadFailureRepo) ListRecords(context.Context, string) ([]*entity.OrderRecord, error) {
	return nil, errors.New("injected receipt read failure")
}

type auditStatusReadFailureRepo struct{ *fakeRepo }

func (r auditStatusReadFailureRepo) ListByStatus(context.Context, string) ([]*entity.Order, error) {
	return nil, errors.New("injected status scan failure")
}

func TestAuditStockRecoveryReportsDatabaseAndRPCFailures(t *testing.T) {
	stock := &fakeStock{}
	s := NewService(auditRecordReadFailureRepo{newFakeRepo()}, &fakeChannel{}, stock, time.Minute, "test-secret", nil)
	if err := s.syncStock(ctx(), "ORD-UNREADABLE", "release"); err == nil || stock.releases != 0 {
		t.Fatalf("must not touch stock without reading persistent receipts: err=%v releases=%d", err, stock.releases)
	}
	s = NewService(auditStatusReadFailureRepo{newFakeRepo()}, &fakeChannel{}, stock, time.Minute, "test-secret", nil)
	if err := s.reconcileStock(ctx()); err == nil {
		t.Fatal("status scan failure must propagate")
	}
	repo := newFakeRepo()
	failingStock := &auditReleaseFailureStock{}
	s = newTestService(repo, &fakeChannel{}, failingStock)
	order := buyerCreate(t, s, "buyer1")
	_, _ = repo.CancelPending(ctx(), order.OrderNo, "取消", time.Now())
	if err := s.reconcileStock(ctx()); err == nil {
		t.Fatal("failed stock reconciliation must be reported")
	}
	s = newTestService(repo, &fakeChannel{}, failingStock)
	if err := s.reconcileStock(ctx()); err != nil || failingStock.releases != 1 {
		t.Fatalf("next scan must recover durable intent: err=%v releases=%d", err, failingStock.releases)
	}
}

type auditRecordWriteFailureRepo struct{ *fakeRepo }

func (r auditRecordWriteFailureRepo) CreateRecord(context.Context, *entity.OrderRecord) error {
	return errors.New("injected record write failure")
}

func TestAuditStockCompletionWriteFailureIsNotAcknowledged(t *testing.T) {
	r := auditRecordWriteFailureRepo{newFakeRepo()}
	stock := &fakeStock{}
	s := NewService(r, &fakeChannel{}, stock, time.Minute, "test-secret", nil)
	if err := s.syncStock(ctx(), "ORD-1", "release"); err == nil {
		t.Fatal("stock completion receipt failure must propagate")
	}
	if stock.releases != 1 {
		t.Fatalf("release calls=%d", stock.releases)
	}
	records, _ := r.ListRecords(ctx(), "ORD-1")
	if len(records) != 0 {
		t.Fatal("failed receipt must not appear persisted")
	}
}

type auditExceptionReadFailureRepo struct{ *fakeRepo }

func (r auditExceptionReadFailureRepo) FindPayExceptionByTradeNo(context.Context, string) (*entity.PayException, error) {
	return nil, errors.New("injected exception read failure")
}

func TestAuditPaymentDoesNotAcknowledgeExceptionLookupFailure(t *testing.T) {
	r := auditExceptionReadFailureRepo{newFakeRepo()}
	stock := &fakeStock{}
	s := NewService(r, &fakeChannel{}, stock, time.Minute, "test-secret", nil)
	o := buyerCreate(t, s, "buyer1")
	now := time.Now().Unix()
	in := NotifyInput{OrderNo: o.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-READ-FAIL", Status: "SUCCESS", NotifiedAtUnix: now}
	in.Signature = sign(in.OrderNo, in.AmountCents, in.ChannelTradeNo, in.Status, now)
	accepted, _ := s.HandlePaymentNotify(ctx(), in)
	order, _ := r.FindByOrderNo(ctx(), o.OrderNo)
	if accepted || order.Status != entity.StatusPendingPayment || stock.confirms != 0 {
		t.Fatalf("lookup failure changed payment: accepted=%v order=%+v confirms=%d", accepted, order, stock.confirms)
	}
}
