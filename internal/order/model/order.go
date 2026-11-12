package model

import "time"

// OrderModel 是 orders 表。channel_trade_no 全局唯一，保证同一渠道交易号只归属一个订单。
type OrderModel struct {
	ID               uint    `gorm:"primaryKey;autoIncrement"`
	OrderNo          string  `gorm:"uniqueIndex;column:order_no;size:40;not null"`
	BuyerID          string  `gorm:"column:buyer_id;size:32;not null;index"`
	ProductID        string  `gorm:"column:product_id;size:32;not null"`
	ProductName      string  `gorm:"column:product_name;size:128;not null"`
	UnitPriceCents   int64   `gorm:"column:unit_price_cents;not null"`
	Quantity         int64   `gorm:"column:quantity;not null"`
	Status           string  `gorm:"column:status;size:24;not null;index"`
	ChannelTradeNo   *string `gorm:"column:channel_trade_no;size:64;uniqueIndex"`
	PaidAt           *time.Time
	Carrier          string `gorm:"column:carrier;size:64"`
	TrackingNo       string `gorm:"column:tracking_no;size:64"`
	ShippedAt        *time.Time
	CancelReason     string `gorm:"column:cancel_reason;size:255"`
	ClosedAt         *time.Time
	RefundFailReason string `gorm:"column:refund_fail_reason;size:255"`
	RefundedAt       *time.Time
	PayExpireAt      time.Time `gorm:"column:pay_expire_at;not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OrderModel) TableName() string {
	return "orders"
}

// OrderRecordModel 是 order_records 表：收款、退款、发货、关闭、异常处理流水。
type OrderRecordModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	OrderNo   string `gorm:"column:order_no;size:40;not null;index"`
	Type      string `gorm:"column:type;size:16;not null"`
	Operator  string `gorm:"column:operator;size:64"`
	Detail    string `gorm:"column:detail;size:512"`
	CreatedAt time.Time
}

func (OrderRecordModel) TableName() string {
	return "order_records"
}

// RefundRequestModel 是 refund_requests 表：售前退款申请。
type RefundRequestModel struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	OrderNo      string `gorm:"column:order_no;size:40;not null;index"`
	BuyerID      string `gorm:"column:buyer_id;size:32;not null"`
	Reason       string `gorm:"column:reason;size:255;not null"`
	Status       string `gorm:"column:status;size:16;not null;index"`
	ReviewerID   string `gorm:"column:reviewer_id;size:32"`
	ReviewReason string `gorm:"column:review_reason;size:255"`
	CreatedAt    time.Time
	ReviewedAt   *time.Time
}

func (RefundRequestModel) TableName() string {
	return "refund_requests"
}

// PayExceptionModel 是 pay_exceptions 表：迟到支付等异常支付记录。
// channel_trade_no 唯一，保证同一笔迟到支付只登记一次异常。
type PayExceptionModel struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	OrderNo        string `gorm:"column:order_no;size:40;not null"`
	ChannelTradeNo string `gorm:"column:channel_trade_no;size:64;uniqueIndex;index:idx_pay_ex_trade,unique"`
	AmountCents    int64  `gorm:"column:amount_cents;not null"`
	Reason         string `gorm:"column:reason;size:64;not null"`
	Status         string `gorm:"column:status;size:16;not null"`
	Detail         string `gorm:"column:detail;size:512"`
	CreatedAt      time.Time
	HandledAt      *time.Time
}

func (PayExceptionModel) TableName() string {
	return "pay_exceptions"
}
