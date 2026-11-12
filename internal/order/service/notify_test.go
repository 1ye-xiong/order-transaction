package service

import (
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

func sign(orderNo string, amount int64, tradeNo string, status string, at int64) string {
	return SignPayload("test-secret", orderNo, amount, tradeNo, status, at)
}

func setupPaidOrder(t *testing.T) (*fakeRepo, *fakeStock, *Service, string) {
	t.Helper()
	repo := newFakeRepo()
	stock := &fakeStock{}
	s := newTestService(repo, &fakeChannel{}, stock)
	order := buyerCreate(t, s, "buyer1")
	return repo, stock, s, order.OrderNo
}

// 验签失败：不接受，并登记可查询的异常记录。
func TestNotifyBadSignature(t *testing.T) {
	repo, _, s, orderNo := setupPaidOrder(t)
	now := time.Now().Unix()
	accepted, reason := s.HandlePaymentNotify(ctx(), NotifyInput{
		OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-BAD", Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: "deadbeef",
	})
	if accepted || reason != "signature invalid" {
		t.Fatalf("accepted = %v, reason = %s", accepted, reason)
	}
	exs, _ := s.ListPayExceptions(ctx(), "")
	if len(exs) != 1 || exs[0].Reason != "signature_invalid" {
		t.Fatalf("异常记录 = %+v", exs)
	}
	o, _ := repo.FindByOrderNo(ctx(), orderNo)
	if o.Status != entity.StatusPendingPayment {
		t.Fatalf("验签失败不应改变状态, got %s", o.Status)
	}
}

// 金额不符：不接受并登记异常。
func TestNotifyAmountMismatch(t *testing.T) {
	repo, _, s, orderNo := setupPaidOrder(t)
	now := time.Now().Unix()
	accepted, _ := s.HandlePaymentNotify(ctx(), NotifyInput{
		OrderNo: orderNo, AmountCents: 9999, ChannelTradeNo: "CH-M", Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: sign(orderNo, 9999, "CH-M", "SUCCESS", now),
	})
	if accepted {
		t.Fatal("金额不符不应接受")
	}
	exs, _ := s.ListPayExceptions(ctx(), "")
	if len(exs) != 1 || exs[0].Reason != "amount_mismatch" {
		t.Fatalf("异常记录 = %+v", exs)
	}
	o, _ := repo.FindByOrderNo(ctx(), orderNo)
	if o.Status != entity.StatusPendingPayment {
		t.Fatalf("金额不符不应改变状态, got %s", o.Status)
	}
}

// 未知订单：登记异常可查询。
func TestNotifyUnknownOrder(t *testing.T) {
	_, _, s, _ := setupPaidOrder(t)
	now := time.Now().Unix()
	accepted, _ := s.HandlePaymentNotify(ctx(), NotifyInput{
		OrderNo: "ORD-UNKNOWN", AmountCents: 10000, ChannelTradeNo: "CH-U", Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: sign("ORD-UNKNOWN", 10000, "CH-U", "SUCCESS", now),
	})
	if accepted {
		t.Fatal("未知订单不应接受")
	}
	exs, _ := s.ListPayExceptions(ctx(), "")
	if len(exs) != 1 || exs[0].Reason != "order_unknown" {
		t.Fatalf("异常记录 = %+v", exs)
	}
}

// 重复通知：同一渠道交易号第二次不重复记款。
func TestNotifyDuplicateIgnored(t *testing.T) {
	repo, stock, s, orderNo := setupPaidOrder(t)
	now := time.Now().Unix()
	in := NotifyInput{OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-DUP", Status: "SUCCESS", NotifiedAtUnix: now}
	in.Signature = sign(orderNo, 10000, "CH-DUP", "SUCCESS", now)
	if accepted, _ := s.HandlePaymentNotify(ctx(), in); !accepted {
		t.Fatal("第一次通知应成功")
	}
	if accepted, reason := s.HandlePaymentNotify(ctx(), in); !accepted || reason != "duplicate ignored" {
		t.Fatalf("重复通知 accepted = %v, reason = %s", accepted, reason)
	}
	o, _ := repo.FindByOrderNo(ctx(), orderNo)
	if stock.confirms != 1 {
		t.Fatalf("confirm 次数 = %d, want 1（重复通知不重复记款）", stock.confirms)
	}
	_ = o
}

// 同一渠道交易号归属另一订单：冲突并登记异常。
func TestNotifyTradeNoConflict(t *testing.T) {
	repo, _, s, order1 := setupPaidOrder(t)
	now := time.Now().Unix()
	in1 := NotifyInput{OrderNo: order1, AmountCents: 10000, ChannelTradeNo: "CH-X", Status: "SUCCESS", NotifiedAtUnix: now}
	in1.Signature = sign(order1, 10000, "CH-X", "SUCCESS", now)
	s.HandlePaymentNotify(ctx(), in1)

	order2DTO := buyerCreate(t, s, "buyer2")
	in2 := NotifyInput{OrderNo: order2DTO.OrderNo, AmountCents: 10000, ChannelTradeNo: "CH-X", Status: "SUCCESS", NotifiedAtUnix: now}
	in2.Signature = sign(order2DTO.OrderNo, 10000, "CH-X", "SUCCESS", now)
	accepted, reason := s.HandlePaymentNotify(ctx(), in2)
	if accepted {
		t.Fatal("交易号冲突不应接受")
	}
	_ = reason
	o2, _ := repo.FindByOrderNo(ctx(), order2DTO.OrderNo)
	if o2.Status == entity.StatusPaid {
		t.Fatal("交易号冲突时订单不应被标记已付款")
	}
}

// 场景 S4：订单超时关闭后收到成功通知 → 迟到支付异常，不复活订单、不重新占用库存。
func TestNotifyLatePayment(t *testing.T) {
	repo, stock, s, orderNo := setupPaidOrder(t)
	repo.mu.Lock()
	repo.orders[orderNo].PayExpireAt = time.Now().Add(-time.Hour)
	repo.mu.Unlock()
	n, _ := s.CloseExpiredOrders(ctx())
	if n != 1 {
		t.Fatalf("预关闭订单, n = %d", n)
	}
	releasesBefore := stock.releases

	now := time.Now().Unix()
	accepted, _ := s.HandlePaymentNotify(ctx(), NotifyInput{
		OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-LATE", Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: sign(orderNo, 10000, "CH-LATE", "SUCCESS", now),
	})
	if !accepted {
		t.Fatal("迟到通知应被受理并进入异常")
	}
	o, _ := repo.FindByOrderNo(ctx(), orderNo)
	if o.Status != entity.StatusClosedTimeout {
		t.Fatalf("迟到支付不应复活订单, status = %s", o.Status)
	}
	if stock.releases != releasesBefore {
		t.Fatal("迟到支付不应重新占用或释放库存")
	}
	if stock.confirms != 0 {
		t.Fatal("迟到支付不应确认扣减")
	}
	exs, _ := s.ListPayExceptions(ctx(), entity.ExceptionPending)
	if len(exs) != 1 || exs[0].Reason != "late_payment" {
		t.Fatalf("异常记录 = %+v", exs)
	}
	// 同一迟到通知重复推送只登记一次
	accepted2, _ := s.HandlePaymentNotify(ctx(), NotifyInput{
		OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-LATE", Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: sign(orderNo, 10000, "CH-LATE", "SUCCESS", now),
	})
	if !accepted2 {
		t.Fatal("重复迟到通知应幂等受理")
	}
	exs2, _ := s.ListPayExceptions(ctx(), entity.ExceptionPending)
	if len(exs2) != 1 {
		t.Fatalf("重复迟到通知不应重复登记, got %d", len(exs2))
	}

	// 客服处理迟到支付退款：渠道退款成功后异常关闭，不恢复库存
	channel := &fakeChannel{}
	s2 := newTestService(repo, channel, stock)
	ex, success, _, err := s2.RefundPayException(ctx(), "cs01", exs[0].ID)
	if err != nil || !success {
		t.Fatalf("RefundPayException: %v / %v", err, success)
	}
	if ex.Status != entity.ExceptionRefunded {
		t.Fatalf("异常状态 = %s, want REFUNDED", ex.Status)
	}
	if stock.restores != 0 {
		t.Fatal("迟到支付退款不应恢复库存（假设 A5）")
	}
}
