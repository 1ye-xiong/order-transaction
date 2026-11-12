package request

// CreateProductRequest 运营创建商品。
type CreateProductRequest struct {
	Name       string `json:"name" binding:"required"`
	PriceCents int64  `json:"price_cents" binding:"required,gt=0"`
	StockTotal int64  `json:"stock_total" binding:"required,gte=0"`
}

// UpdateProductRequest 运营调整商品：售价、上下架、补货。
type UpdateProductRequest struct {
	SetPrice   bool  `json:"set_price"`
	PriceCents int64 `json:"price_cents"`
	SetOnSale  bool  `json:"set_on_sale"`
	OnSale     bool  `json:"on_sale"`
	StockDelta int64 `json:"stock_delta"`
}

// CreateOrderRequest 买家下单。
type CreateOrderRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int64  `json:"quantity" binding:"required,gte=1,lte=2"`
}

// ApplyRefundRequest 买家申请售前退款。
type ApplyRefundRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// ShipRequest 仓库发货登记。
type ShipRequest struct {
	Carrier    string `json:"carrier" binding:"required"`
	TrackingNo string `json:"tracking_no" binding:"required"`
}

// ReviewRefundRequest 客服审核退款申请。
type ReviewRefundRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// ChannelConfigRequest 渠道行为配置（教学模拟控制台）。
type ChannelConfigRequest struct {
	PayBehavior              string `json:"pay_behavior" binding:"required,oneof=success fail no_notify"`
	NotifyCount              int32  `json:"notify_count" binding:"gte=1,lte=10"`
	NotifyDelaySeconds       int32  `json:"notify_delay_seconds" binding:"gte=0"`
	PayLateSeconds           int32  `json:"pay_late_seconds" binding:"gte=0"`
	RefundSuccessFromAttempt int32  `json:"refund_success_from_attempt" binding:"gte=1"`
}

// CreateOrderResponse 下单返回。
type CreateOrderResponse struct {
	OrderNo string `json:"order_no"`
}
