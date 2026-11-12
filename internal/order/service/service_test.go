package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

func buyerCreate(t *testing.T, s *Service, buyerID string) *dto.OrderDTO {
	t.Helper()
	order, err := s.CreateOrder(context.Background(), dto.CreateCommand{BuyerID: buyerID, ProductID: "P1", Quantity: 1})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	return order
}

func payNotify(t *testing.T, s *Service, orderNo string, tradeNo string) bool {
	t.Helper()
	accepted, _ := s.HandlePaymentNotify(context.Background(), NotifyInput{
		OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: tradeNo, Status: "SUCCESS",
		NotifiedAtUnix: time.Now().Unix(),
		Signature:      SignPayload("test-secret", orderNo, 10000, tradeNo, "SUCCESS", time.Now().Unix()),
	})
	return accepted
}

// 场景 S1：下单 → 支付通知 → PAID，库存确认扣减一次；发货一次。
func TestCreateAndPayFlow(t *testing.T) {
	repo := newFakeRepo()
	stock := &fakeStock{}
	s := newTestService(repo, &fakeChannel{}, stock)
	ctx := context.Background()

	order := buyerCreate(t, s, "buyer1")
	if order.AmountCents != 10000 {
		t.Fatalf("amount = %d, want 10000", order.AmountCents)
	}
	if order.Status != entity.StatusPendingPayment {
		t.Fatalf("status = %s", order.Status)
	}
	if _, err := s.PayOrder(ctx, "buyer1", order.OrderNo); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if !payNotify(t, s, order.OrderNo, "CH-1") {
		t.Fatal("支付通知应成功")
	}
	updated, _ := repo.FindByOrderNo(ctx, order.OrderNo)
	if updated.Status != entity.StatusPaid {
		t.Fatalf("status = %s, want PAID", updated.Status)
	}
	if stock.confirms != 1 {
		t.Fatalf("confirm 次数 = %d, want 1", stock.confirms)
	}
	shipped, err := s.ShipOrder(ctx, dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF001"})
	if err != nil || shipped.Status != entity.StatusShipped {
		t.Fatalf("ship: %v / %s", err, shipped.Status)
	}
	if _, err := s.ShipOrder(ctx, dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF001"}); err == nil {
		t.Fatal("重复发货应失败")
	}
}

// 场景 S2：取消/超时关闭释放占用，重复关闭不多释放。
func TestCancelAndTimeoutReleaseOnce(t *testing.T) {
	repo := newFakeRepo()
	stock := &fakeStock{}
	s := newTestService(repo, &fakeChannel{}, stock)
	ctx := context.Background()

	order := buyerCreate(t, s, "buyer1")
	if _, err := s.CancelOrder(ctx, "buyer1", order.OrderNo, "不要了"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := s.CancelOrder(ctx, "buyer1", order.OrderNo, "不要了"); err != nil {
		t.Fatalf("cancel twice: %v", err)
	}
	if stock.releases != 1 {
		t.Fatalf("release 次数 = %d, want 1", stock.releases)
	}

	// 超时关闭：把有效期改到过去，模拟服务端时间判断
	order2 := buyerCreate(t, s, "buyer2")
	repo.mu.Lock()
	repo.orders[order2.OrderNo].PayExpireAt = time.Now().Add(-time.Minute)
	repo.mu.Unlock()

	n, err := s.CloseExpiredOrders(ctx)
	if err != nil || n != 1 {
		t.Fatalf("close expired: n = %d, err = %v", n, err)
	}
	_, _ = s.CloseExpiredOrders(ctx)
	if stock.releases != 2 {
		t.Fatalf("超时后 release 总次数 = %d, want 2（重复扫描不多释放）", stock.releases)
	}
	o2, _ := repo.FindByOrderNo(ctx, order2.OrderNo)
	if o2.Status != entity.StatusClosedTimeout {
		t.Fatalf("status = %s", o2.Status)
	}
}

// 越权：买家只能操作本人订单。
func TestOwnershipEnforced(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	ctx := context.Background()
	order := buyerCreate(t, s, "buyer1")
	if _, err := s.GetOrder(ctx, "buyer2", order.OrderNo); err != entity.ErrNotOrderOwner {
		t.Fatalf("他人查询应 ErrNotOrderOwner, got %v", err)
	}
	if _, err := s.CancelOrder(ctx, "buyer2", order.OrderNo, "x"); err != entity.ErrNotOrderOwner {
		t.Fatalf("他人取消应 ErrNotOrderOwner, got %v", err)
	}
}

// 数量限制：每单 1~2 件。
func TestQuantityValidation(t *testing.T) {
	s := newTestService(newFakeRepo(), &fakeChannel{}, &fakeStock{})
	if _, err := s.CreateOrder(context.Background(), dto.CreateCommand{BuyerID: "buyer1", ProductID: "P1", Quantity: 3}); err != entity.ErrInvalidQuantity {
		t.Fatalf("quantity=3 应失败, got %v", err)
	}
}

// 下单库存占用失败时不得创建订单。
func TestCreateOrderReserveFails(t *testing.T) {
	repo := newFakeRepo()
	stock := &fakeStock{reserveErr: context.DeadlineExceeded}
	s := newTestService(repo, &fakeChannel{}, stock)
	if _, err := s.CreateOrder(context.Background(), dto.CreateCommand{BuyerID: "buyer1", ProductID: "P1", Quantity: 1}); err == nil {
		t.Fatal("占用失败应返回错误")
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.orders) != 0 {
		t.Fatal("占用失败不应创建订单")
	}
}

// 场景 S5：退款申请与发货并发——申请后发货被拒绝。
func TestShipBlockedWhileRefundRequested(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	ctx := context.Background()
	order := buyerCreate(t, s, "buyer1")
	_, _ = s.PayOrder(ctx, "buyer1", order.OrderNo)
	if !payNotify(t, s, order.OrderNo, "CH-2") {
		t.Fatal("notify 应成功")
	}
	if _, err := s.ApplyRefund(ctx, dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "不想要了"}); err != nil {
		t.Fatalf("apply refund: %v", err)
	}
	_, err := s.ShipOrder(ctx, dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF"})
	if err == nil || !strings.Contains(err.Error(), "shipping paused") {
		t.Fatalf("退款申请期间发货应被拒绝, got %v", err)
	}
}

// 场景 S3：客服同意 → 首次退款失败 → 重试成功 → 恢复可售数量一次。
func TestRefundFailThenRetrySuccess(t *testing.T) {
	repo := newFakeRepo()
	stock := &fakeStock{}
	channel := &fakeChannel{succeedFrom: 2}
	s := newTestService(repo, channel, stock)
	ctx := context.Background()
	order := buyerCreate(t, s, "buyer1")
	_, _ = s.PayOrder(ctx, "buyer1", order.OrderNo)
	if !payNotify(t, s, order.OrderNo, "CH-3") {
		t.Fatal("notify 应成功")
	}
	_, _ = s.ApplyRefund(ctx, dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "商品有瑕疵"})

	reqs, _ := s.ListRefundRequests(ctx, entity.RefundPending)
	if len(reqs) != 1 {
		t.Fatalf("pending refunds = %d", len(reqs))
	}
	if _, err := s.ReviewRefund(ctx, dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: reqs[0].ID, Approve: true, Reason: "同意"}); err != nil {
		t.Fatalf("review: %v", err)
	}
	o, _ := repo.FindByOrderNo(ctx, order.OrderNo)
	if o.Status != entity.StatusRefundFailed {
		t.Fatalf("首次退款失败后 status = %s", o.Status)
	}
	if stock.restores != 0 {
		t.Fatal("退款失败不应恢复库存")
	}
	if _, err := s.RetryRefund(ctx, "cs01", order.OrderNo); err != nil {
		t.Fatalf("retry: %v", err)
	}
	o, _ = repo.FindByOrderNo(ctx, order.OrderNo)
	if o.Status != entity.StatusRefunded {
		t.Fatalf("重试成功后 status = %s", o.Status)
	}
	if stock.restores != 1 {
		t.Fatalf("restore 次数 = %d, want 1（重复退款不重复恢复）", stock.restores)
	}
}

// 拒绝退款后恢复可发货。
func TestRefundRejectRestoresShipping(t *testing.T) {
	repo := newFakeRepo()
	s := newTestService(repo, &fakeChannel{}, &fakeStock{})
	ctx := context.Background()
	order := buyerCreate(t, s, "buyer1")
	_, _ = s.PayOrder(ctx, "buyer1", order.OrderNo)
	if !payNotify(t, s, order.OrderNo, "CH-4") {
		t.Fatal("notify 应成功")
	}
	_, _ = s.ApplyRefund(ctx, dto.ApplyRefundCommand{BuyerID: "buyer1", OrderNo: order.OrderNo, Reason: "误操作"})
	reqs, _ := s.ListRefundRequests(ctx, entity.RefundPending)
	if _, err := s.ReviewRefund(ctx, dto.ReviewRefundCommand{ReviewerID: "cs01", RefundID: reqs[0].ID, Approve: false, Reason: "不符合退款政策"}); err != nil {
		t.Fatalf("review reject: %v", err)
	}
	o, _ := repo.FindByOrderNo(ctx, order.OrderNo)
	if o.Status != entity.StatusPaid {
		t.Fatalf("拒绝后 status = %s, want PAID", o.Status)
	}
	shipped, err := s.ShipOrder(ctx, dto.ShipCommand{Operator: "wh", OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF9"})
	if err != nil || shipped.Status != entity.StatusShipped {
		t.Fatalf("拒绝后应可发货: %v", err)
	}
}
