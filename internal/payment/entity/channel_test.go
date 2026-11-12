package entity

import "testing"

func TestChannelConfigNormalize(t *testing.T) {
	c := ChannelConfig{}
	c.Normalize()
	if c.PayBehavior != PayBehaviorSuccess || c.NotifyCount != 1 || c.RefundSuccessFromAttempt != 1 {
		t.Fatalf("默认配置错误: %+v", c)
	}
	c = ChannelConfig{PayBehavior: PayBehaviorNoNotify, NotifyCount: 99, NotifyDelaySeconds: 5, PayLateSeconds: 3, RefundSuccessFromAttempt: 7}
	c.Normalize()
	if c.NotifyCount != 10 {
		t.Fatalf("NotifyCount 应截断为 10, got %d", c.NotifyCount)
	}
	if c.PayBehavior != PayBehaviorNoNotify || c.RefundSuccessFromAttempt != 7 {
		t.Fatalf("合法配置不应被修改: %+v", c)
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultChannelConfig()
	if c.PayBehavior != PayBehaviorSuccess || c.NotifyCount != 1 || c.PayLateSeconds != 0 || c.RefundSuccessFromAttempt != 1 {
		t.Fatalf("默认配置 = %+v", c)
	}
}

func TestRecordTypeConstants(t *testing.T) {
	if RecordTypePayment != "PAYMENT" || RecordTypeRefund != "REFUND" {
		t.Fatal("流水类型常量异常")
	}
}
