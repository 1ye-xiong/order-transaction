package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
)

// fakeNotify 记录通知投递。
type fakeNotify struct {
	mu      sync.Mutex
	calls   []NotifyRequest
	changed chan struct{}
	err     error
}

func (f *fakeNotify) SendPaymentNotify(_ context.Context, req NotifyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	if f.changed != nil {
		close(f.changed)
		f.changed = nil
	}
	return f.err
}

func (f *fakeNotify) snapshot() []NotifyRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]NotifyRequest(nil), f.calls...)
}

func (f *fakeNotify) waitCalls(t *testing.T, n int) []NotifyRequest {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		f.mu.Lock()
		if len(f.calls) >= n {
			calls := append([]NotifyRequest(nil), f.calls...)
			f.mu.Unlock()
			return calls
		}
		if f.changed == nil {
			f.changed = make(chan struct{})
		}
		changed := f.changed
		f.mu.Unlock()
		select {
		case <-changed:
		case <-timer.C:
			t.Fatalf("通知等待超时，收到%d次，预期%d次", len(f.snapshot()), n)
			return nil
		}
	}
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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = c
	return nil
}
func (r *fakeChannelRepo) LoadConfig(_ context.Context) (entity.ChannelConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.config, nil
}
func (r *fakeChannelRepo) SaveRecord(_ context.Context, record *entity.ChannelRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	record.ID = r.seq
	if record.Type == entity.RecordTypeRefund {
		r.attempts[record.RefundNo]++
	}
	r.records = append(r.records, *record)
	return nil
}
func (r *fakeChannelRepo) ListRecords(_ context.Context) ([]*entity.ChannelRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var items []*entity.ChannelRecord
	for i := range r.records {
		cp := r.records[i]
		items = append(items, &cp)
	}
	return items, nil
}
func (r *fakeChannelRepo) CountRefundAttempts(_ context.Context, refundNo string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	calls := notifier.waitCalls(t, 1)
	if len(calls) != 1 {
		t.Fatalf("通知次数 = %d, want 1", len(calls))
	}
	call := calls[0]
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
	calls := notifier.waitCalls(t, 3)
	if len(calls) != 3 {
		t.Fatalf("通知次数 = %d, want 3", len(calls))
	}
}

// 支付失败与超时不通知场景。
func TestPayFailAndNoNotify(t *testing.T) {
	repo := newFakeChannelRepo()
	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorFail, NotifyCount: 1})
	notifier := &fakeNotify{}
	s := NewService(repo, notifier, "secret", nil)
	_, _ = s.Pay(context.Background(), "ORD-F", "buyer1", 1000)
	calls := notifier.waitCalls(t, 1)
	if len(calls) != 1 || calls[0].Status != NotifyStatusFailed {
		t.Fatalf("失败场景通知 = %+v", calls)
	}

	_ = repo.SaveConfig(context.Background(), entity.ChannelConfig{PayBehavior: entity.PayBehaviorNoNotify, NotifyCount: 1})
	_, _ = s.Pay(context.Background(), "ORD-N", "buyer1", 1000)
	// Observe a bounded quiet period using the notifier's synchronized event signal.
	notifier.mu.Lock()
	if len(notifier.calls) != 1 {
		notifier.mu.Unlock()
		t.Fatal("no_notify produced an extra notification")
	}
	if notifier.changed == nil {
		notifier.changed = make(chan struct{})
	}
	changed := notifier.changed
	notifier.mu.Unlock()
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-changed:
		t.Fatalf("no_notify 场景不应发送通知, calls = %d", len(notifier.snapshot()))
	case <-timer.C:
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

// errChannelRepo 在指定仓储操作上注入失败，覆盖服务层的错误传播分支。
type errChannelRepo struct {
	fakeChannelRepo
	errLoadConfig    error
	errSaveRecord    error
	errCountAttempts error
}

func (r *errChannelRepo) LoadConfig(_ context.Context) (entity.ChannelConfig, error) {
	if r.errLoadConfig != nil {
		return entity.ChannelConfig{}, r.errLoadConfig
	}
	return r.fakeChannelRepo.LoadConfig(context.Background())
}

func (r *errChannelRepo) SaveRecord(ctx context.Context, record *entity.ChannelRecord) error {
	if r.errSaveRecord != nil {
		return r.errSaveRecord
	}
	return r.fakeChannelRepo.SaveRecord(ctx, record)
}

func (r *errChannelRepo) CountRefundAttempts(_ context.Context, refundNo string) (int, error) {
	if r.errCountAttempts != nil {
		return 0, r.errCountAttempts
	}
	return r.fakeChannelRepo.CountRefundAttempts(context.Background(), refundNo)
}

// atomicChannelRepo 实现仓储的原子退款扩展接口，覆盖 ExecuteRefund 快速路径。
type atomicChannelRepo struct {
	fakeChannelRepo
	calls int
}

func (r *atomicChannelRepo) ExecuteRefund(_ context.Context, refundNo string, _ string, _ int64, _ string) (bool, string, error) {
	r.calls++
	return true, "atomic refund " + refundNo, nil
}

var errInjected = errors.New("注入的仓储故障")

func TestPayLoadConfigError(t *testing.T) {
	s := NewService(&errChannelRepo{errLoadConfig: errInjected}, &fakeNotify{}, "secret", nil)
	if _, err := s.Pay(context.Background(), "ORD-E1", "buyer1", 1000); !errors.Is(err, errInjected) {
		t.Fatalf("LoadConfig 失败应向上传播, got %v", err)
	}
}

func TestPaySaveRecordError(t *testing.T) {
	s := NewService(&errChannelRepo{errSaveRecord: errInjected}, &fakeNotify{}, "secret", nil)
	if _, err := s.Pay(context.Background(), "ORD-E2", "buyer1", 1000); !errors.Is(err, errInjected) {
		t.Fatalf("SaveRecord 失败应向上传播, got %v", err)
	}
}

// 通知投递失败只记录告警，不影响受理结果，且按 NotifyCount 重试。
func TestDispatchNotifyFailureRetries(t *testing.T) {
	notifier := &fakeNotify{err: errInjected}
	s := NewService(newFakeChannelRepo(), notifier, "secret", nil)
	cfg := entity.ChannelConfig{PayBehavior: entity.PayBehaviorSuccess, NotifyCount: 3}
	// 同步触发投递过程，断言重试次数与告警路径（异步路径由 Pay 系列测试覆盖）。
	s.dispatchNotify("ORD-E3", 1000, "CH-E3", cfg)
	if got := len(notifier.snapshot()); got != cfg.NotifyCount {
		t.Fatalf("重试%d次, 收到%d次", cfg.NotifyCount, got)
	}
}

// 仓储支持原子退款时直接走 ExecuteRefund，不再走“计数+判定”路径。
func TestRefundAtomicFastPath(t *testing.T) {
	repo := &atomicChannelRepo{fakeChannelRepo: *newFakeChannelRepo()}
	s := NewService(repo, &fakeNotify{}, "secret", nil)
	ok, msg, err := s.Refund(context.Background(), "RF-A", "CH-A", 10000, "退款")
	if err != nil || !ok || msg != "atomic refund RF-A" {
		t.Fatalf("原子退款快速路径 = (%v, %s, %v)", ok, msg, err)
	}
	if repo.calls != 1 {
		t.Fatalf("ExecuteRefund 调用次数 = %d, want 1", repo.calls)
	}
	if records := repo.records; len(records) != 0 {
		t.Fatalf("快速路径不应写渠道流水, got %d 条", len(records))
	}
}

func TestRefundLoadConfigError(t *testing.T) {
	s := NewService(&errChannelRepo{errLoadConfig: errInjected}, &fakeNotify{}, "secret", nil)
	if _, _, err := s.Refund(context.Background(), "RF-E1", "CH", 1000, "退款"); !errors.Is(err, errInjected) {
		t.Fatalf("LoadConfig 失败应向上传播, got %v", err)
	}
}

func TestRefundCountAttemptsError(t *testing.T) {
	s := NewService(&errChannelRepo{errCountAttempts: errInjected}, &fakeNotify{}, "secret", nil)
	if _, _, err := s.Refund(context.Background(), "RF-E2", "CH", 1000, "退款"); !errors.Is(err, errInjected) {
		t.Fatalf("CountRefundAttempts 失败应向上传播, got %v", err)
	}
}

func TestRefundSaveRecordError(t *testing.T) {
	s := NewService(&errChannelRepo{errSaveRecord: errInjected}, &fakeNotify{}, "secret", nil)
	if _, _, err := s.Refund(context.Background(), "RF-E3", "CH", 1000, "退款"); !errors.Is(err, errInjected) {
		t.Fatalf("SaveRecord 失败应向上传播, got %v", err)
	}
}

func TestGetConfigError(t *testing.T) {
	s := NewService(&errChannelRepo{errLoadConfig: errInjected}, &fakeNotify{}, "secret", nil)
	if _, err := s.GetConfig(context.Background()); !errors.Is(err, errInjected) {
		t.Fatalf("GetConfig 错误应向上传播, got %v", err)
	}
}

func TestListRecords(t *testing.T) {
	repo := newFakeChannelRepo()
	s := NewService(repo, &fakeNotify{}, "secret", nil)
	_, _ = s.Pay(context.Background(), "ORD-L1", "buyer1", 1000)
	records, err := s.ListRecords(context.Background())
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("受理后应有渠道流水")
	}
	if records[0].OrderNo != "ORD-L1" || records[0].Result != "success" {
		t.Fatalf("流水内容 = %+v", records[0])
	}
}
