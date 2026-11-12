package entity

import (
	"testing"
	"time"
)

func TestOrderAmount(t *testing.T) {
	o := Order{UnitPriceCents: 10000, Quantity: 2}
	if o.AmountCents() != 20000 {
		t.Fatalf("amount = %d", o.AmountCents())
	}
}

func TestOrderStatePredicates(t *testing.T) {
	now := time.Now()
	future := now.Add(10 * time.Minute)
	past := now.Add(-10 * time.Minute)
	pending := Order{Status: StatusPendingPayment, PayExpireAt: future}
	if !pending.CanPay(now) || pending.CanCancel() == false || pending.CanApplyRefund() {
		t.Fatal("待付款订单判定错误")
	}
	expired := Order{Status: StatusPendingPayment, PayExpireAt: past}
	if expired.CanPay(now) || !expired.IsExpired(now) {
		t.Fatal("过期订单判定错误")
	}
	paid := Order{Status: StatusPaid}
	if !paid.CanApplyRefund() || !paid.Shippable() {
		t.Fatal("待发货订单判定错误")
	}
	shipped := Order{Status: StatusShipped}
	if shipped.CanApplyRefund() || shipped.Shippable() {
		t.Fatal("已发货订单不应可退款或可发货")
	}
}

func TestStatusConstants(t *testing.T) {
	want := []string{StatusPendingPayment, StatusPaid, StatusShipped, StatusCancelled, StatusClosedTimeout, StatusRefundRequested, StatusRefundFailed, StatusRefunded}
	seen := map[string]bool{}
	for _, s := range want {
		if s == "" || seen[s] {
			t.Fatalf("状态常量异常: %s", s)
		}
		seen[s] = true
	}
}
