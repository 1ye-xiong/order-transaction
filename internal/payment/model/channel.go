package model

import "time"

// ChannelConfigModel 持久化渠道行为配置（单行表）。
type ChannelConfigModel struct {
	ID                       uint   `gorm:"primaryKey;autoIncrement"`
	PayBehavior              string `gorm:"column:pay_behavior;size:16;not null"`
	NotifyCount              int    `gorm:"column:notify_count;not null"`
	NotifyDelaySeconds       int    `gorm:"column:notify_delay_seconds;not null"`
	PayLateSeconds           int    `gorm:"column:pay_late_seconds;not null"`
	RefundSuccessFromAttempt int    `gorm:"column:refund_success_from_attempt;not null"`
	UpdatedAt                time.Time
}

func (ChannelConfigModel) TableName() string {
	return "channel_config"
}

// ChannelRecordModel 是 channel_records 表：渠道受理流水。
type ChannelRecordModel struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	Type           string `gorm:"column:type;size:16;not null;index"`
	ChannelTradeNo string `gorm:"column:channel_trade_no;size:64;index"`
	RefundNo       string `gorm:"column:refund_no;size:64;index"`
	OrderNo        string `gorm:"column:order_no;size:40;not null;index"`
	AmountCents    int64  `gorm:"column:amount_cents;not null"`
	Result         string `gorm:"column:result;size:32;not null"`
	Attempts       int    `gorm:"column:attempts;not null;default:1"`
	Detail         string `gorm:"column:detail;size:512"`
	CreatedAt      time.Time
}

func (ChannelRecordModel) TableName() string {
	return "channel_records"
}
