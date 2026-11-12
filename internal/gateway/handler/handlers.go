package handler

import (
	"go.uber.org/zap"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
)

// Handlers 聚合题目 01 的网关 handler。
type Handlers struct {
	Product *ProductHandler
	Order   *OrderHandler
	Staff   *StaffHandler
	Channel *ChannelHandler
}

func NewHandlers(product productv1.ProductServiceClient, order orderv1.OrderServiceClient, payment paymentv1.PaymentServiceClient, staff *StaffHandler, log *zap.Logger) *Handlers {
	return &Handlers{
		Product: NewProductHandler(product, log),
		Order:   NewOrderHandler(order, log),
		Staff:   staff,
		Channel: NewChannelHandler(payment, log),
	}
}
