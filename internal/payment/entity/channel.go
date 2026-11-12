package entity

import (
	"context"
	"errors"
	"time"
)

var (
	ErrChannelRejected = errors.New("channel rejected")
	ErrRecordNotFound  = errors.New("channel record not found")
)

// 渠道支付行为配置值。
const (
	PayBehaviorSuccess  = "success"
	PayBehaviorFail     = "fail"
	PayBehaviorNoNotify = "no_notify"

	RecordTypePayment = "PAYMENT"
	RecordTypeRefund  = "REFUND"
)

// ChannelConfig 是模拟支付渠道的全局行为配置（教学用）。
type ChannelConfig struct {
	PayBehavior              string
	NotifyCount              int
	NotifyDelaySeconds       int
	PayLateSeconds           int
	RefundSuccessFromAttempt int
}

// DefaultChannelConfig 返回默认配置：支付成功、通知一次、无延迟。
func DefaultChannelConfig() ChannelConfig {
	return ChannelConfig{
		PayBehavior:              PayBehaviorSuccess,
		NotifyCount:              1,
		NotifyDelaySeconds:       0,
		PayLateSeconds:           0,
		RefundSuccessFromAttempt: 1,
	}
}

// Normalize 修正配置越界值。
func (c *ChannelConfig) Normalize() {
	if c.PayBehavior != PayBehaviorSuccess && c.PayBehavior != PayBehaviorFail && c.PayBehavior != PayBehaviorNoNotify {
		c.PayBehavior = PayBehaviorSuccess
	}
	if c.NotifyCount < 1 {
		c.NotifyCount = 1
	}
	if c.NotifyCount > 10 {
		c.NotifyCount = 10
	}
	if c.NotifyDelaySeconds < 0 {
		c.NotifyDelaySeconds = 0
	}
	if c.PayLateSeconds < 0 {
		c.PayLateSeconds = 0
	}
	if c.RefundSuccessFromAttempt < 1 {
		c.RefundSuccessFromAttempt = 1
	}
}

// ChannelRecord 是渠道受理流水（支付/退款），持久化用于核查。
type ChannelRecord struct {
	ID             uint
	Type           string
	ChannelTradeNo string
	RefundNo       string
	OrderNo        string
	AmountCents    int64
	Result         string
	Attempts       int
	Detail         string
	CreatedAt      time.Time
}

// Repository 是渠道流水的仓储接口。
type Repository interface {
	SaveConfig(ctx context.Context, config ChannelConfig) error
	LoadConfig(ctx context.Context) (ChannelConfig, error)
	SaveRecord(ctx context.Context, record *ChannelRecord) error
	ListRecords(ctx context.Context) ([]*ChannelRecord, error)
	CountRefundAttempts(ctx context.Context, refundNo string) (int, error)
}
