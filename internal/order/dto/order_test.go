package dto

import (
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

func TestFromOrder(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	expire := now.Add(10 * time.Minute)
	paid := now
	order := &entity.Order{
		OrderNo: "ORD-1", BuyerID: "buyer1", ProductID: "P1", ProductName: "商品",
		UnitPriceCents: 10000, Quantity: 2, Status: entity.StatusPaid,
		ChannelTradeNo: "CH-1", PaidAt: &paid, Carrier: "SF", TrackingNo: "SF001",
		CancelReason: "", RefundFailReason: "channel busy",
		PayExpireAt: expire, CreatedAt: now, UpdatedAt: now,
	}
	got := FromOrder(order)
	if got.AmountCents != 20000 {
		t.Fatalf("amount = %d", got.AmountCents)
	}
	if got.PaidAt != "2026-01-02 03:04:05" || got.PayExpireAt == "" || got.RefundFailReason != "channel busy" {
		t.Fatalf("dto = %+v", got)
	}
	// 空指针时间应输出空串
	empty := FromOrder(&entity.Order{UnitPriceCents: 1, Quantity: 1, PayExpireAt: expire})
	if empty.PaidAt != "" || empty.ShippedAt != "" {
		t.Fatalf("空时间应输出空串: %+v", empty)
	}
}

func TestFromOrders(t *testing.T) {
	now := time.Now()
	items := FromOrders([]*entity.Order{{OrderNo: "A", UnitPriceCents: 1, Quantity: 1, CreatedAt: now}, {OrderNo: "B", UnitPriceCents: 1, Quantity: 1, CreatedAt: now}})
	if len(items) != 2 || items[0].OrderNo != "A" || items[1].OrderNo != "B" {
		t.Fatalf("items = %+v", items)
	}
	if len(FromOrders(nil)) != 0 {
		t.Fatal("空列表应返回空切片")
	}
}
