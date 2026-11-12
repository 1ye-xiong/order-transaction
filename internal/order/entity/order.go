package entity

import (
	"context"
	"errors"
	"time"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidQuantity    = errors.New("invalid quantity")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidState       = errors.New("invalid order state")
	ErrNotOrderOwner      = errors.New("not order owner")
	ErrSignatureInvalid   = errors.New("signature invalid")
	ErrAmountMismatch     = errors.New("amount mismatch")
	ErrOrderUnknown       = errors.New("order unknown")
	ErrTradeNoConflict    = errors.New("channel trade no belongs to another order")
	ErrReasonRequired     = errors.New("reason required")
	ErrTrackingNoRequired = errors.New("carrier and tracking no required")
)

// 订单状态机（题目 01）：
// PENDING_PAYMENT -> PAID(有效支付通知) -> SHIPPED / REFUND_REQUESTED -> REFUNDED / REFUND_FAILED
// PENDING_PAYMENT -> CANCELLED(买家取消) / CLOSED_TIMEOUT(超时)
// CLOSED_TIMEOUT + 迟到成功通知 -> pay_exceptions（异常支付，不复活订单）
const (
	StatusPendingPayment  = "PENDING_PAYMENT"
	StatusPaid            = "PAID"
	StatusShipped         = "SHIPPED"
	StatusCancelled       = "CANCELLED"
	StatusClosedTimeout   = "CLOSED_TIMEOUT"
	StatusRefundRequested = "REFUND_REQUESTED"
	StatusRefundFailed    = "REFUND_FAILED"
	StatusRefunded        = "REFUNDED"
)

// 记录类型（运营核查流水）。
const (
	RecordTypePay       = "PAY"
	RecordTypeClose     = "CLOSE"
	RecordTypeRefund    = "REFUND"
	RecordTypeShip      = "SHIP"
	RecordTypeException = "EXCEPTION"
)

// 退款申请与异常支付状态。
const (
	RefundPending  = "PENDING"
	RefundApproved = "APPROVED"
	RefundRejected = "REJECTED"

	ExceptionPending    = "PENDING"
	ExceptionProcessing = "PROCESSING"
	ExceptionRefunded   = "REFUNDED"
)

// Order 是订单聚合。金额一律使用人民币分。
type Order struct {
	OrderNo          string
	BuyerID          string
	ProductID        string
	ProductName      string
	UnitPriceCents   int64
	Quantity         int64
	Status           string
	ChannelTradeNo   string
	PaidAt           *time.Time
	Carrier          string
	TrackingNo       string
	ShippedAt        *time.Time
	CancelReason     string
	ClosedAt         *time.Time
	RefundFailReason string
	RefundedAt       *time.Time
	PayExpireAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// AmountCents 订单金额（下单时单价 x 数量，后续调价不影响）。
func (o *Order) AmountCents() int64 {
	return o.UnitPriceCents * o.Quantity
}

// IsExpired 用服务端时间判断待付款订单是否超时。
func (o *Order) IsExpired(now time.Time) bool {
	return o.Status == StatusPendingPayment && now.After(o.PayExpireAt)
}

// CanCancel 待付款订单可被本人取消。
func (o *Order) CanCancel() bool { return o.Status == StatusPendingPayment }

// CanPay 待付款订单且未超时可发起支付。
func (o *Order) CanPay(now time.Time) bool {
	return o.Status == StatusPendingPayment && !now.After(o.PayExpireAt)
}

// CanApplyRefund 已付款未发货订单可申请售前退款。
func (o *Order) CanApplyRefund() bool { return o.Status == StatusPaid }

// Shippable 待发货状态（退款申请期间暂停发货）。
func (o *Order) Shippable() bool { return o.Status == StatusPaid }

// OrderRecord 是订单操作流水，供运营核查收款、退款、发货、关闭与异常处理。
type OrderRecord struct {
	ID        uint
	OrderNo   string
	Type      string
	Operator  string
	Detail    string
	CreatedAt time.Time
}

// RefundRequest 是买家的售前退款申请。
type RefundRequest struct {
	ID           uint
	OrderNo      string
	BuyerID      string
	Reason       string
	Status       string
	ReviewerID   string
	ReviewReason string
	CreatedAt    time.Time
	ReviewedAt   *time.Time
}

// PayException 是异常支付记录（迟到支付、验签失败、金额不符、未知订单）。
type PayException struct {
	ID             uint
	OrderNo        string
	ChannelTradeNo string
	AmountCents    int64
	Reason         string
	Status         string
	Detail         string
	CreatedAt      time.Time
	HandledAt      *time.Time
}

// Repository 是订单域仓储接口。所有状态变更使用事务内条件更新保证幂等与并发正确。
type Repository interface {
	Create(ctx context.Context, order *Order) error
	FindByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	// CancelPending 条件更新 PENDING_PAYMENT -> CANCELLED，返回是否本次执行成功（重复取消幂等）。
	CancelPending(ctx context.Context, orderNo string, reason string, now time.Time) (bool, error)
	// CloseIfExpired 条件更新 PENDING_PAYMENT -> CLOSED_TIMEOUT（仅当服务端时间已过期），返回是否成功。
	CloseIfExpired(ctx context.Context, orderNo string, now time.Time) (bool, error)
	// MarkPaid 条件更新 PENDING_PAYMENT -> PAID 并登记渠道交易号；交易号冲突或状态不符返回失败。
	MarkPaid(ctx context.Context, orderNo string, tradeNo string, now time.Time) (bool, error)
	// Transition 条件更新 from -> to 并应用字段修改，返回是否成功。
	Transition(ctx context.Context, orderNo string, from string, to string, apply func(*Order), now time.Time) (bool, error)
	ListByBuyer(ctx context.Context, buyerID string, offset int, limit int) ([]*Order, int64, error)
	ListByStatus(ctx context.Context, status string) ([]*Order, error)
	ListExpiredPending(ctx context.Context, now time.Time) ([]*Order, error)
	// 流水
	CreateRecord(ctx context.Context, record *OrderRecord) error
	ListRecords(ctx context.Context, orderNo string) ([]*OrderRecord, error)
	// 退款申请
	CreateRefundRequest(ctx context.Context, req *RefundRequest) error
	FindRefundRequest(ctx context.Context, id uint) (*RefundRequest, error)
	FindPendingRefundByOrder(ctx context.Context, orderNo string) (*RefundRequest, error)
	// FindLatestRefundByOrder 返回订单最近一次退款申请（供重试退款定位退款单号）。
	FindLatestRefundByOrder(ctx context.Context, orderNo string) (*RefundRequest, error)
	UpdateRefundRequest(ctx context.Context, req *RefundRequest) error
	ListRefundRequests(ctx context.Context, status string) ([]*RefundRequest, error)
	// 异常支付
	CreatePayException(ctx context.Context, ex *PayException) error
	FindPayException(ctx context.Context, id uint) (*PayException, error)
	FindPayExceptionByTradeNo(ctx context.Context, tradeNo string) (*PayException, error)
	UpdatePayException(ctx context.Context, ex *PayException) error
	ListPayExceptions(ctx context.Context, status string) ([]*PayException, error)
	// 汇总
	Summary(ctx context.Context) (paidCents int64, refundedCents int64, counts map[string]int64, exceptionCount int64, err error)
}
