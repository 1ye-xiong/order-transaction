package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
	paymentrepo "github.com/BwCloudWeGo/bw-cli/internal/payment/repo"
	paymentservice "github.com/BwCloudWeGo/bw-cli/internal/payment/service"
)

type nopNotifier struct{}

func (nopNotifier) SendPaymentNotify(context.Context, paymentservice.NotifyRequest) error { return nil }

func newTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := paymentrepo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := paymentservice.NewService(paymentrepo.NewGormRepository(db), nopNotifier{}, "test-secret", zap.NewNop())
	return NewServer(svc, zap.NewNop())
}

func TestChannelConfigHandlers(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	set, err := s.SetChannelConfig(ctx, &paymentv1.SetChannelConfigRequest{
		PayBehavior:              entity.PayBehaviorNoNotify,
		NotifyCount:              99,
		NotifyDelaySeconds:       -3,
		PayLateSeconds:           -1,
		RefundSuccessFromAttempt: 2,
	})
	if err != nil {
		t.Fatalf("set config: %v", err)
	}
	if set.PayBehavior != entity.PayBehaviorNoNotify || set.RefundSuccessFromAttempt != 2 {
		t.Fatalf("set response = %+v", set)
	}

	got, err := s.GetChannelConfig(ctx, &paymentv1.GetChannelConfigRequest{})
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if got.PayBehavior != entity.PayBehaviorNoNotify {
		t.Fatalf("behavior = %s", got.PayBehavior)
	}
	if got.NotifyCount != 10 || got.NotifyDelaySeconds != 0 || got.PayLateSeconds != 0 {
		t.Fatalf("越界值未归一化: %+v", got)
	}

	if _, err := s.SetChannelConfig(ctx, &paymentv1.SetChannelConfigRequest{PayBehavior: "unknown"}); err != nil {
		t.Fatalf("set invalid behavior: %v", err)
	}
	got, err = s.GetChannelConfig(ctx, &paymentv1.GetChannelConfigRequest{})
	if err != nil || got.PayBehavior != entity.PayBehaviorSuccess {
		t.Fatalf("非法行为应回落默认值: %v / %+v", err, got)
	}
}

func TestPayHandler(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	if _, err := s.SetChannelConfig(ctx, &paymentv1.SetChannelConfigRequest{PayBehavior: entity.PayBehaviorNoNotify, RefundSuccessFromAttempt: 1}); err != nil {
		t.Fatalf("set config: %v", err)
	}

	resp, err := s.Pay(ctx, &paymentv1.PayRequest{OrderNo: "O-1", BuyerId: "buyer1", AmountCents: 10000})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if !resp.Accepted || !strings.HasPrefix(resp.ChannelTradeNo, "CH") || resp.Message != "accepted" {
		t.Fatalf("pay response = %+v", resp)
	}
}

func TestRefundHandler(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	if _, err := s.SetChannelConfig(ctx, &paymentv1.SetChannelConfigRequest{PayBehavior: entity.PayBehaviorNoNotify, RefundSuccessFromAttempt: 2}); err != nil {
		t.Fatalf("set config: %v", err)
	}

	first, err := s.Refund(ctx, &paymentv1.RefundRequest{RefundNo: "R-1", ChannelTradeNo: "CH-1", AmountCents: 10000, Reason: "user"})
	if err != nil {
		t.Fatalf("refund 1: %v", err)
	}
	if first.Success || !strings.Contains(first.Message, "attempt 1") {
		t.Fatalf("首次退款应失败: %+v", first)
	}

	second, err := s.Refund(ctx, &paymentv1.RefundRequest{RefundNo: "R-1", ChannelTradeNo: "CH-1", AmountCents: 10000, Reason: "user"})
	if err != nil {
		t.Fatalf("refund 2: %v", err)
	}
	if !second.Success || !strings.Contains(second.Message, "attempt 2") {
		t.Fatalf("第二次退款应成功: %+v", second)
	}
}

func TestListChannelRecords(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()
	if _, err := s.Pay(ctx, &paymentv1.PayRequest{OrderNo: "O-1", BuyerId: "buyer1", AmountCents: 10000}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := s.Refund(ctx, &paymentv1.RefundRequest{RefundNo: "R-1", ChannelTradeNo: "CH-1", AmountCents: 10000, Reason: "user"}); err != nil {
		t.Fatalf("refund: %v", err)
	}

	resp, err := s.ListChannelRecords(ctx, &paymentv1.ListChannelRecordsRequest{})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("records = %d", len(resp.Items))
	}
	var payment, refund *paymentv1.ChannelRecordDTO
	for _, item := range resp.Items {
		switch item.Type {
		case entity.RecordTypePayment:
			payment = item
		case entity.RecordTypeRefund:
			refund = item
		}
	}
	if payment == nil || payment.Id == "" || payment.OrderNo != "O-1" || payment.AmountCents != 10000 {
		t.Fatalf("payment record = %+v", payment)
	}
	if payment.CreatedAt == "" || payment.ChannelTradeNo == "" {
		t.Fatalf("record 缺少时间或流水号: %+v", payment)
	}
	if refund == nil || refund.RefundNo != "R-1" || refund.Attempts != 1 {
		t.Fatalf("refund record = %+v", refund)
	}
}

// failingRepo 让仓储全部报错，用于覆盖 handler 的错误分支。
type failingRepo struct{}

var errRepoDown = errors.New("repo down")

func (failingRepo) SaveConfig(context.Context, entity.ChannelConfig) error { return errRepoDown }
func (failingRepo) LoadConfig(context.Context) (entity.ChannelConfig, error) {
	return entity.ChannelConfig{}, errRepoDown
}
func (failingRepo) SaveRecord(context.Context, *entity.ChannelRecord) error { return errRepoDown }
func (failingRepo) ListRecords(context.Context) ([]*entity.ChannelRecord, error) {
	return nil, errRepoDown
}
func (failingRepo) CountRefundAttempts(context.Context, string) (int, error) { return 0, errRepoDown }

func TestHandlerErrorBranches(t *testing.T) {
	s := NewServer(paymentservice.NewService(failingRepo{}, nopNotifier{}, "test-secret", zap.NewNop()), zap.NewNop())
	ctx := context.Background()

	if _, err := s.SetChannelConfig(ctx, &paymentv1.SetChannelConfigRequest{}); err == nil {
		t.Fatal("set config 应返回仓储错误")
	}
	if _, err := s.GetChannelConfig(ctx, &paymentv1.GetChannelConfigRequest{}); err == nil {
		t.Fatal("get config 应返回仓储错误")
	}
	pay, err := s.Pay(ctx, &paymentv1.PayRequest{OrderNo: "O-1", AmountCents: 10000})
	if err != nil || pay.Accepted || pay.Message == "" {
		t.Fatalf("pay 失败应体现在响应中: %v / %+v", err, pay)
	}
	refund, err := s.Refund(ctx, &paymentv1.RefundRequest{RefundNo: "R-1", AmountCents: 10000})
	if err != nil || refund.Success || refund.Message == "" {
		t.Fatalf("refund 失败应体现在响应中: %v / %+v", err, refund)
	}
	if _, err := s.ListChannelRecords(ctx, &paymentv1.ListChannelRecordsRequest{}); err == nil {
		t.Fatal("list records 应返回仓储错误")
	}
}
