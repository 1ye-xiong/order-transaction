package service

import (
	"context"
	"sync"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
	"testing"
)

// fakeNotify 记录通知投递。
type fakeNotify struct {
	calls []NotifyRequest
}

func (f *fakeNotify) SendPaymentNotify(_ context.Context, req NotifyRequest) error {
	f.calls = append(f.calls, req)
	return nil
}

// fakeChannelRepo 是渠道仓储的内存实现。
type fakeChannelRepo struct {
	mu       sync.Mutex
	config   entity.ChannelConfig
	records  []entity.ChannelRecord
	attempts map[string]int
	seq      uint
}

func newFakeChannelRepo() *fakeChannelRepo {
	return &fakeChannelRepo{config: entity.DefaultChannelConfig(), attempts: map[string]int{}}
}

func (r *fakeChannelRepo) SaveConfig(_ context.Context, c entity.ChannelConfig) error {
	r.config = c
	return nil
}
func (r *fakeChannelRepo) LoadConfig(_ context.Context) (entity.ChannelConfig, error) {
	return r.config, nil
}
func (r *fakeChannelRepo) SaveRecord(_ context.Context, record *entity.ChannelRecord) error {
	r.seq++
	record.ID = r.seq
	if record.Type == entity.RecordTypeRefund {
		r.attempts[record.RefundNo]++
	}
	r.records = append(r.records, *record)
	return nil
}
func (r *fakeChannelRepo) ListRecords(_ context.Context) ([]*entity.ChannelRecord, error) {
	var items []*entity.ChannelRecord
	for i := range r.records {
		cp := r.records[i]
		items = append(items, &cp)
	}
	return items, nil
}
func (r *fakeChannelRepo) CountRefundAttempts(_ context.Context, refundNo string) (int, error) {
	return r.attempts[refundNo], nil
}

// Sign 与 Verify 互验。
func TestSignAndVerify(t *testing.T) {
	s := NewService(newFakeChannelRepo(), &fakeNotify{}, "secret", nil)
	sig := s.Sign("ORD-1", 10000, "CH-1", "SUCCESS", 1700000000)
	if !s.Verify("ORD-1", 10000, "CH-1", "SUCCESS", 1700000000, sig) {
		t.Fatal("合法签名应通过校验")
	}
	if s.Verify("ORD-1", 10000, "CH-1", "SUCCESS", 1700000001, sig) {
		t.Fatal("时间戳篡改应失败")
	}
	if s.Verify("ORD-1", 10001, "CH-1", "SUCCESS", 1700000000, sig) {
		t.Fatal("金额篡改应失败")
	}
}

// 渠道配置归一化。
func TestConfigNormalize(t *testing.T) {
	c := entity.ChannelConfig{PayBehavior: "invalid", NotifyCount: 0, NotifyDelaySeconds: -1, PayLateSeconds: -1, RefundSuccessFromAttempt: 0}
	c.Normalize()
	if c.PayBehavior != entity.PayBehaviorSuccess || c.NotifyCount != 1 || c.RefundSuccessFromAttempt != 1 {
		t.Fatalf("Normalize 结果 = %+v", c)
	}
	if c.NotifyCount > 10 {
		t.Fatal("通知次数应被限制")
	}
}

// Pay 受理后按配置发送成功通知。
func TestPaySuccessNotify(t *testing.T) {
	repo := newFakeChannelRepo()
	notifier := &fakeNotify{}
	s := NewService(repo, notifier, "secret", nil)
	tradeNo, err := s.Pay(context.Background(), "ORD-1", "buyer1", 10000)
	time.Sleep(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("通知次数 = %d, want 1", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.OrderNo != "ORD-1" || call.Status != NotifyStatusSuccess || call.ChannelTradeNo != tradeNo {
		t.Fatalf("通知内容 = %+v", call)
	}
	if !s.Verify(call.OrderNo, call.AmountCents, call.ChannelTradeNo, call.Status, call.NotifiedAtUnix, call.Signature) {
		t.Fatal("通知签名应可验证")
	}
}

// 重复通知场景：NotifyCount=3 时发送三次。
func TestPayDuplicateNotify(t *testing.T) {
	repo := newFakeChannelRepo()
	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorSuccess, NotifyCount: 3})
	notifier := &fakeNotify{}
	s := NewService(repo, notifier, "secret", nil)
	_, _ = s.Pay(context.Background(), "ORD-DUP", "buyer1", 5000)
	time.Sleep(150 * time.Millisecond)
	if len(notifier.calls) != 3 {
		t.Fatalf("通知次数 = %d, want 3", len(notifier.calls))
	}
}

// 支付失败与超时不通知场景。
func TestPayFailAndNoNotify(t *testing.T) {
	repo := newFakeChannelRepo()
	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorFail, NotifyCount: 1})
	notifier := &fakeNotify{}
	s := NewService(repo, notifier, "secret", nil)
	_, _ = s.Pay(context.Background(), "ORD-F", "buyer1", 1000)
	time.Sleep(100 * time.Millisecond)
	if len(notifier.calls) != 1 || notifier.calls[0].Status != NotifyStatusFailed {
		t.Fatalf("失败场景通知 = %+v", notifier.calls)
	}

	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorNoNotify, NotifyCount: 1})
	_, _ = s.Pay(context.Background(), "ORD-N", "buyer1", 1000)
	time.Sleep(100 * time.Millisecond)
	if len(notifier.calls) != 1 {
		t.Fatalf("no_notify 场景不应发送通知, calls = %d", len(notifier.calls))
	}
}

// 退款：首次失败、再次成功（refund_success_from_attempt=2）。
func TestRefundFirstFailThenSuccess(t *testing.T) {
	repo := newFakeChannelRepo()
	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorSuccess, RefundSuccessFromAttempt: 2})
	s := NewService(repo, &fakeNotify{}, "secret", nil)

	ok1, msg1, err := s.Refund(context.Background(), "RF-1", "CH-1", 10000, "退款")
	if err != nil || ok1 {
		t.Fatalf("首次退款应失败: ok = %v, err = %v, msg = %s", ok1, err, msg1)
	}
	ok2, _, err := s.Refund(context.Background(), "RF-1", "CH-1", 10000, "退款")
	if err != nil || !ok2 {
		t.Fatalf("第二次退款应成功: ok = %v, err = %v", ok2, err)
	}
	records, _ := repo.ListRecords(context.Background())
	var refundRecords int
	for _, r := range records {
		if r.Type == entity.RecordTypeRefund {
			refundRecords++
		}
	}
	if refundRecords != 2 {
		t.Fatalf("退款流水 = %d, want 2", refundRecords)
	}
}

// 渠道配置读写。
func TestSetAndGetConfig(t *testing.T) {
	s := NewService(newFakeChannelRepo(), &fakeNotify{}, "secret", nil)
	err := s.SetConfig(context.Background(), entity.ChannelConfig{
		PayBehavior: entity.PayBehaviorFail, NotifyCount: 2, RefundSuccessFromAttempt: 3,
	})
	if err != nil {
		t.Fatalf("set config: %v", err)
	}
	c, err := s.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if c.PayBehavior != entity.PayBehaviorFail || c.NotifyCount != 2 || c.RefundSuccessFromAttempt != 3 {
		t.Fatalf("config = %+v", c)
	}
}
