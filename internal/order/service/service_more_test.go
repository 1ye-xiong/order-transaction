package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// refundErrChannel 让渠道退款直接报错，用于覆盖退款异常分支。
type refundErrChannel struct{ *fakeChannel }

func (refundErrChannel) Refund(context.Context, string, string, int64, string) (bool, string, error) {
	return false, "", errors.New("channel unavailable")
}

func markPaidOrder(t *testing.T, s *Service, orderNo string, tradeNo string) {
	t.Helper()
	if _, err := s.PayOrder(ctx(), "buyer1", orderNo); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if !payNotify(t, s, orderNo, tradeNo) {
		t.Fatalf("notify %s 应成功", orderNo)
	}
}

func TestListMyOrdersAndPagination(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	for i := 0; i < 3; i++ {
		buyerCreate(t, s, "buyer1")
	}
	_, _ = s.CreateOrder(context.Background(), dto.CreateCommand{BuyerID: "buyer2", ProductID: "P1", Quantity: 1})

	orders, total, err := s.ListMyOrders(ctx(), "buyer1", 1, 2)
	if err != nil || total != 3 || len(orders) != 3 {
		t.Fatalf("list: %v / total = %d / len = %d", err, total, len(orders))
	}
	// 分页参数归一化：page<=0 与 pageSize<=0 走默认值，pageSize>100 截断
	if _, _, err := s.ListMyOrders(ctx(), "buyer1", 0, 0); err != nil {
		t.Fatalf("默认分页: %v", err)
	}
	if _, _, err := s.ListMyOrders(ctx(), "buyer1", -1, 500); err != nil {
		t.Fatalf("超界分页: %v", err)
	}
	if offset, limit := normalizePagination(0, 0); offset != 0 || limit != 20 {
		t.Fatalf("normalizePagination(0,0) = %d,%d", offset, limit)
	}
	if offset, limit := normalizePagination(3, 500); offset != 200 || limit != 100 {
		t.Fatalf("normalizePagination(3,500) = %d,%d", offset, limit)
	}
}

func TestListShippableRecordsAndSummary(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")
	markPaidOrder(t, s, order.OrderNo, "CH-L1")

	shippable, err := s.ListShippableOrders(ctx())
	if err != nil || len(shippable) != 1 || shippable[0].OrderNo != order.OrderNo {
		t.Fatalf("shippable: %v / %+v", err, shippable)
	}
	paid, refunded, counts, exceptions, err := s.OpsSummary(ctx())
	if err != nil || paid != 10000 || refunded != 0 || exceptions != 0 {
		t.Fatalf("summary: %v / %d / %d / %d", err, paid, refunded, exceptions)
	}
	if counts[entity.StatusPaid] != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	records, err := s.ListOrderRecords(ctx(), order.OrderNo)
	if err != nil || len(records) < 2 {
		t.Fatalf("records: %v / %d", err, len(records))
	}
}

func TestGetOrderNotFound(t *testing.T) {
	s := newTestService(newFakeRepo(), &fakeChannel{}, &fakeStock{})
	if _, err := s.GetOrder(ctx(), "buyer1", "ORD-NONE"); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单应 ErrOrderNotFound, got %v", err)
	}
}

func TestPayOrderRejections(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")

	if _, err := s.PayOrder(ctx(), "buyer2", order.OrderNo); err != entity.ErrNotOrderOwner {
		t.Fatalf("他人支付应 ErrNotOrderOwner, got %v", err)
	}
	if _, err := s.PayOrder(ctx(), "buyer1", "ORD-NONE"); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单支付应 ErrOrderNotFound, got %v", err)
	}

	markPaidOrder(t, s, order.OrderNo, "CH-P1")
	if _, err := s.PayOrder(ctx(), "buyer1", order.OrderNo); err == nil || !strings.Contains(err.Error(), "not payable") {
		t.Fatalf("已支付订单不可再次支付, got %v", err)
	}

	// 渠道报错
	failing := newTestService(repo, &fakeChannel{payErr: errors.New("channel down")}, &fakeStock{})
	order2 := buyerCreate(t, failing, "buyer1")
	if _, err := failing.PayOrder(ctx(), "buyer1", order2.OrderNo); err == nil {
		t.Fatal("渠道失败应返回错误")
	}

	// 超时订单：支付前先按服务端时间关闭
	timeoutSvc := NewService(newFakeRepo(), &fakeChannel{}, &fakeStock{}, -time.Minute, "test-secret", nil)
	expired := buyerCreate(t, timeoutSvc, "buyer1")
	if _, err := timeoutSvc.PayOrder(ctx(), "buyer1", expired.OrderNo); err == nil || !strings.Contains(err.Error(), "pay timeout") {
		t.Fatalf("过期订单支付应失败, got %v", err)
	}
	closed, _ := timeoutSvc.GetOrder(ctx(), "buyer1", expired.OrderNo)
	if closed.Status != entity.StatusClosedTimeout {
		t.Fatalf("过期订单状态 = %s", closed.Status)
	}
}

func TestShipRejectedForNonPaidStatus(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")
	if _, err := s.CancelOrder(ctx(), "buyer1", order.OrderNo, "取消"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := s.ShipOrder(ctx(), dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF1"}); err == nil || !strings.Contains(err.Error(), "not shippable") {
		t.Fatalf("已取消订单不可发货, got %v", err)
	}
	if _, err := s.ShipOrder(ctx(), dto.ShipCommand{Operator: "wh", OrderNo: "ORD-NONE", Carrier: "SF", TrackingNo: "SF1"}); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单发货应 ErrOrderNotFound, got %v", err)
	}
}

func TestApplyRefundValidationBranches(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")

	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo}); err != entity.ErrReasonRequired {
		t.Fatalf("空原因应 ErrReasonRequired, got %v", err)
	}
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: "ORD-NONE", Reason: "x"}); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单应 ErrOrderNotFound, got %v", err)
	}
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer2", OrderNo: order.OrderNo, Reason: "x"}); err != entity.ErrNotOrderOwner {
		t.Fatalf("他人订单应 ErrNotOrderOwner, got %v", err)
	}
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "待付款不可退"}); err == nil {
		t.Fatal("待付款订单不可申请退款")
	}

	markPaidOrder(t, s, order.OrderNo, "CH-R1")
	first, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不想要"})
	if err != nil || first.Status != entity.StatusRefundRequested {
		t.Fatalf("apply: %v / %+v", err, first)
	}
	// 申请中再次申请：状态不可退，直接拒绝
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不想要"}); err == nil {
		t.Fatal("退款申请中不可重复申请")
	}
	// 已有待审核申请时（订单被人工改回 PAID）直接返回，不重复建单
	repo.mu.Lock()
	repo.orders[order.OrderNo].Status = entity.StatusPaid
	repo.mu.Unlock()
	again, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不想要"})
	if err != nil || again.Status != entity.StatusPaid {
		t.Fatalf("已有待审核申请应幂等返回: %v / %+v", err, again)
	}
	pending, _ := s.ListRefundRequests(ctx(), entity.RefundPending)
	if len(pending) != 1 {
		t.Fatalf("不应重复建单, pending = %d", len(pending))
	}
}

func TestReviewRefundErrorsAndChannelFailure(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	if _, err := s.ReviewRefund(ctx(), dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: 404, Approve: true}); err != entity.ErrOrderNotFound {
		t.Fatalf("未知退款申请应 ErrOrderNotFound, got %v", err)
	}

	order := buyerCreate(t, s, "buyer1")
	markPaidOrder(t, s, order.OrderNo, "CH-RV")
	if _, err := s.ApplyRefund(ctx(), dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "瑕疵"}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	reqs, _ := s.ListRefundRequests(ctx(), entity.RefundPending)
	if len(reqs) != 1 {
		t.Fatalf("pending = %d", len(reqs))
	}

	// 订单状态被人工改回 PAID 时，审核拒绝继续
	repo.mu.Lock()
	repo.orders[order.OrderNo].Status = entity.StatusPaid
	repo.mu.Unlock()
	if _, err := s.ReviewRefund(ctx(), dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: reqs[0].ID, Approve: true}); err == nil {
		t.Fatal("订单不在退款审核状态时应拒绝")
	}
	repo.mu.Lock()
	repo.orders[order.OrderNo].Status = entity.StatusRefundRequested
	repo.mu.Unlock()

	// 渠道退款报错：审核同意后返回错误，订单保持申请中
	failing := newTestService(repo, refundErrChannel{&fakeChannel{}}, &fakeStock{})
	if _, err := failing.ReviewRefund(ctx(), dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: reqs[0].ID, Approve: true, Reason: "同意"}); err == nil {
		t.Fatal("渠道退款失败应返回错误")
	}
	// 已审核的申请不可重复审核
	if _, err := s.ReviewRefund(ctx(), dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: reqs[0].ID, Approve: true, Reason: "同意"}); err == nil || !strings.Contains(err.Error(), "already reviewed") {
		t.Fatalf("重复审核应失败, got %v", err)
	}
}

func TestRetryRefundErrors(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	if _, err := s.RetryRefund(ctx(), "cs01", "ORD-NONE"); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单应 ErrOrderNotFound, got %v", err)
	}

	order := buyerCreate(t, s, "buyer1")
	if _, err := s.RetryRefund(ctx(), "cs01", order.OrderNo); err == nil || !strings.Contains(err.Error(), "refund-failed") {
		t.Fatalf("非退款失败订单不可重试, got %v", err)
	}

	// 状态为 REFUND_FAILED 但找不到退款申请
	repo.mu.Lock()
	repo.orders[order.OrderNo].Status = entity.StatusRefundFailed
	repo.mu.Unlock()
	if _, err := s.RetryRefund(ctx(), "cs01", order.OrderNo); err == nil || !strings.Contains(err.Error(), "refund request not found") {
		t.Fatalf("缺少退款申请应报错, got %v", err)
	}
}

func TestExecuteRefundNoTransitionTarget(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")
	markPaidOrder(t, s, order.OrderNo, "CH-EX")
	if _, err := s.ShipOrder(ctx(), dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF1"}); err != nil {
		t.Fatalf("ship: %v", err)
	}
	// 已发货订单既非 REFUND_REQUESTED 也非 REFUND_FAILED，退款成功后状态不变
	result, err := s.executeRefund(ctx(), order.OrderNo, "RF-EX", "cs01", "测试")
	if err != nil {
		t.Fatalf("executeRefund: %v", err)
	}
	if result.Status != entity.StatusShipped {
		t.Fatalf("状态不应改变, got %s", result.Status)
	}
}

func TestNotifyFailedAndInvalidState(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	order := buyerCreate(t, s, "buyer1")

	now := time.Now().Unix()
	failed := NotifyInput{OrderNo: order.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-F", Status: "FAILED", NotifiedAtUnix: now}
	failed.Signature = sign(order.OrderNo, 10000, "CH-F", "FAILED", now)
	accepted, reason := s.HandlePaymentNotify(ctx(), failed)
	if !accepted || reason != "failed notify recorded" {
		t.Fatalf("失败通知: %v / %s", accepted, reason)
	}
	current, _ := s.GetOrder(ctx(), "buyer1", order.OrderNo)
	if current.Status != entity.StatusPendingPayment {
		t.Fatalf("失败通知不应改变状态, got %s", current.Status)
	}

	markPaidOrder(t, s, order.OrderNo, "CH-OK")
	if _, err := s.ShipOrder(ctx(), dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF1"}); err != nil {
		t.Fatalf("ship: %v", err)
	}
	now2 := time.Now().Unix()
	invalid := NotifyInput{OrderNo: order.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-OK2", Status: "SUCCESS", NotifiedAtUnix: now2}
	invalid.Signature = sign(order.OrderNo, 10000, "CH-OK2", "SUCCESS", now2)
	accepted, reason = s.HandlePaymentNotify(ctx(), invalid)
	if accepted || reason != "invalid state" {
		t.Fatalf("已发货订单收到成功通知: %v / %s", accepted, reason)
	}
	exs, _ := s.ListPayExceptions(ctx(), entity.ExceptionPending)
	if len(exs) != 1 || exs[0].Reason != "invalid_state" {
		t.Fatalf("异常记录 = %+v", exs)
	}
}

func TestRefundPayExceptionBranches(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	if _, _, _, err := s.RefundPayException(ctx(), "cs01", 404); err == nil {
		t.Fatal("未知异常应报错")
	}

	ex := &entity.PayException{OrderNo: "ORD-X", ChannelTradeNo: "CH-X", AmountCents: 10000, Reason: "late_payment", Status: entity.ExceptionPending}
	if err := repo.CreatePayException(ctx(), ex); err != nil {
		t.Fatalf("create exception: %v", err)
	}

	// 渠道退款报错
	failing := newTestService(repo, refundErrChannel{&fakeChannel{}}, &fakeStock{})
	if _, _, _, err := failing.RefundPayException(ctx(), "cs01", ex.ID); err == nil {
		t.Fatal("渠道错误应返回 error")
	}

	// 渠道退款失败：保持待处理并记录原因
	failRefund := newTestService(repo, &fakeChannel{succeedFrom: 2}, &fakeStock{})
	got, success, message, err := failRefund.RefundPayException(ctx(), "cs01", ex.ID)
	if err != nil || success || message == "" || got.Status != entity.ExceptionPending {
		t.Fatalf("首次退款失败: %v / %v / %s / %s", err, success, message, got.Status)
	}

	// 已处理异常不可重复退款
	if _, success, msg, err := failRefund.RefundPayException(ctx(), "cs01", 999); err == nil || success || msg != "" {
		t.Fatalf("未知异常: %v / %v / %s", err, success, msg)
	}
	done := &entity.PayException{OrderNo: "ORD-Y", ChannelTradeNo: "CH-Y", AmountCents: 100, Status: entity.ExceptionRefunded}
	_ = repo.CreatePayException(ctx(), done)
	if _, success, msg, err := failRefund.RefundPayException(ctx(), "cs01", done.ID); err != nil || success || !strings.Contains(msg, "already handled") {
		t.Fatalf("已处理异常: %v / %v / %s", err, success, msg)
	}
}
