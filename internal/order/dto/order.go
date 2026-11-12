package dto

import (
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// OrderDTO 是订单用例层返回的数据结构。
type OrderDTO struct {
	OrderNo          string
	BuyerID          string
	ProductID        string
	ProductName      string
	UnitPriceCents   int64
	Quantity         int64
	AmountCents      int64
	Status           string
	ChannelTradeNo   string
	PaidAt           string
	Carrier          string
	TrackingNo       string
	ShippedAt        string
	CancelReason     string
	ClosedAt         string
	RefundFailReason string
	RefundedAt       string
	PayExpireAt      string
	CreatedAt        string
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func FromOrder(order *entity.Order) *OrderDTO {
	return &OrderDTO{
		OrderNo:          order.OrderNo,
		BuyerID:          order.BuyerID,
		ProductID:        order.ProductID,
		ProductName:      order.ProductName,
		UnitPriceCents:   order.UnitPriceCents,
		Quantity:         order.Quantity,
		AmountCents:      order.AmountCents(),
		Status:           order.Status,
		ChannelTradeNo:   order.ChannelTradeNo,
		PaidAt:           formatTime(order.PaidAt),
		Carrier:          order.Carrier,
		TrackingNo:       order.TrackingNo,
		ShippedAt:        formatTime(order.ShippedAt),
		CancelReason:     order.CancelReason,
		ClosedAt:         formatTime(order.ClosedAt),
		RefundFailReason: order.RefundFailReason,
		RefundedAt:       formatTime(order.RefundedAt),
		PayExpireAt:      order.PayExpireAt.Format(time.RFC3339),
		CreatedAt:        order.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func FromOrders(orders []*entity.Order) []*OrderDTO {
	items := make([]*OrderDTO, 0, len(orders))
	for _, order := range orders {
		items = append(items, FromOrder(order))
	}
	return items
}

// CreateCommand 买家下单。
type CreateCommand struct {
	BuyerID   string
	ProductID string
	Quantity  int64
}

// ApplyRefundCommand 买家申请售前退款。
type ApplyRefundCommand struct {
	BuyerID string
	OrderNo string
	Reason  string
}

// ShipCommand 仓库发货登记。
type ShipCommand struct {
	Operator   string
	OrderNo    string
	Carrier    string
	TrackingNo string
}

// ReviewRefundCommand 客服审核退款申请。
type ReviewRefundCommand struct {
	ReviewerID string
	RefundID   uint
	Approve    bool
	Reason     string
}
