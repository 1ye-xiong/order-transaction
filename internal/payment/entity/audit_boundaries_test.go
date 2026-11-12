package entity

import "testing"

func TestAuditChannelNegativeDelaysAndValidFailureBehavior(t *testing.T) {
	c := ChannelConfig{PayBehavior: PayBehaviorFail, NotifyCount: 1, NotifyDelaySeconds: -1, PayLateSeconds: -2, RefundSuccessFromAttempt: 2}
	c.Normalize()
	if c.PayBehavior != PayBehaviorFail || c.NotifyDelaySeconds != 0 || c.PayLateSeconds != 0 || c.NotifyCount != 1 || c.RefundSuccessFromAttempt != 2 {
		t.Fatalf("negative delays must be zero and valid failure settings preserved: %+v", c)
	}
}
