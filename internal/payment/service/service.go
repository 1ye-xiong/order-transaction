package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
)

// Notifier 把支付通知投递给订单服务，由 handler/main 注入真实 gRPC client，便于单元测试。
type Notifier interface {
	SendPaymentNotify(ctx context.Context, req NotifyRequest) error
}

// NotifyRequest 是渠道发往商城的支付通知载荷。
type NotifyRequest struct {
	OrderNo        string
	AmountCents    int64
	ChannelTradeNo string
	Status         string
	NotifiedAtUnix int64
	Signature      string
}

// NotifyStatus 通知中的支付结果。
const (
	NotifyStatusSuccess = "SUCCESS"
	NotifyStatusFailed  = "FAILED"
)

// Service 编排模拟渠道用例。
type Service struct {
	repo     entity.Repository
	notifier Notifier
	secret   string
	log      *zap.Logger
}

func NewService(repo entity.Repository, notifier Notifier, secret string, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, notifier: notifier, secret: secret, log: log}
}

// Sign 生成通知签名：HMAC-SHA256(order_no|amount|trade_no|status|notified_at)。
func (s *Service) Sign(orderNo string, amountCents int64, tradeNo string, status string, notifiedAt int64) string {
	payload := orderNo + "|" + strconv.FormatInt(amountCents, 10) + "|" + tradeNo + "|" + status + "|" + strconv.FormatInt(notifiedAt, 10)
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify 校验通知签名。
func (s *Service) Verify(orderNo string, amountCents int64, tradeNo string, status string, notifiedAt int64, signature string) bool {
	expected := s.Sign(orderNo, amountCents, tradeNo, status, notifiedAt)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Pay 受理付款：记录流水并按渠道配置异步发出通知（重复/失败/超时不通知/迟到均由配置控制）。
func (s *Service) Pay(ctx context.Context, orderNo string, buyerID string, amountCents int64) (string, error) {
	config, err := s.repo.LoadConfig(ctx)
	if err != nil {
		return "", err
	}
	tradeNo, err := newTradeNo()
	if err != nil {
		return "", err
	}

	record := &entity.ChannelRecord{
		Type:           entity.RecordTypePayment,
		ChannelTradeNo: tradeNo,
		OrderNo:        orderNo,
		AmountCents:    amountCents,
		Result:         "accepted",
		Detail:         fmt.Sprintf("buyer=%s behavior=%s", buyerID, config.PayBehavior),
	}
	if err := s.repo.SaveRecord(ctx, record); err != nil {
		return "", err
	}

	if config.PayBehavior == entity.PayBehaviorNoNotify {
		s.log.Info("channel accept without notify", zap.String("order_no", orderNo), zap.String("trade_no", tradeNo))
		return tradeNo, nil
	}

	go s.dispatchNotify(orderNo, amountCents, tradeNo, config)
	return tradeNo, nil
}

// dispatchNotify 按配置延迟后发送通知；NotifyCount>1 时重复发送（网络重试场景）。
func (s *Service) dispatchNotify(orderNo string, amountCents int64, tradeNo string, config entity.ChannelConfig) {
	delay := time.Duration(config.NotifyDelaySeconds)*time.Second + time.Duration(config.PayLateSeconds)*time.Second
	time.Sleep(delay)

	status := NotifyStatusSuccess
	if config.PayBehavior == entity.PayBehaviorFail {
		status = NotifyStatusFailed
	}
	for i := 0; i < config.NotifyCount; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		now := time.Now().Unix()
		req := NotifyRequest{
			OrderNo:        orderNo,
			AmountCents:    amountCents,
			ChannelTradeNo: tradeNo,
			Status:         status,
			NotifiedAtUnix: now,
			Signature:      s.Sign(orderNo, amountCents, tradeNo, status, now),
		}
		if err := s.notifier.SendPaymentNotify(ctx, req); err != nil {
			s.log.Warn("payment notify failed",
				zap.String("order_no", orderNo),
				zap.String("trade_no", tradeNo),
				zap.Int("attempt", i+1),
				zap.Error(err),
			)
		} else {
			s.log.Info("payment notify delivered",
				zap.String("order_no", orderNo),
				zap.String("trade_no", tradeNo),
				zap.String("status", status),
				zap.Int("attempt", i+1),
			)
		}
		cancel()
	}
}

// Refund 受理退款：第 N 次尝试起成功（N 来自配置），返回即时结果。
func (s *Service) Refund(ctx context.Context, refundNo string, channelTradeNo string, amountCents int64, reason string) (bool, string, error) {
	config, err := s.repo.LoadConfig(ctx)
	if err != nil {
		return false, "", err
	}
	attempts, err := s.repo.CountRefundAttempts(ctx, refundNo)
	if err != nil {
		return false, "", err
	}
	attempt := attempts + 1
	success := attempt >= config.RefundSuccessFromAttempt

	result := "failed"
	message := fmt.Sprintf("refund attempt %d failed", attempt)
	if success {
		result = "success"
		message = fmt.Sprintf("refund attempt %d success", attempt)
	}

	record := &entity.ChannelRecord{
		Type:           entity.RecordTypeRefund,
		ChannelTradeNo: channelTradeNo,
		RefundNo:       refundNo,
		OrderNo:        "",
		AmountCents:    amountCents,
		Result:         result,
		Attempts:       attempt,
		Detail:         reason,
	}
	if err := s.repo.SaveRecord(ctx, record); err != nil {
		return false, "", err
	}
	return success, message, nil
}

func (s *Service) SetConfig(ctx context.Context, config entity.ChannelConfig) error {
	return s.repo.SaveConfig(ctx, config)
}

func (s *Service) GetConfig(ctx context.Context) (entity.ChannelConfig, error) {
	config, err := s.repo.LoadConfig(ctx)
	if err != nil {
		return entity.ChannelConfig{}, err
	}
	config.Normalize()
	return config, nil
}

func (s *Service) ListRecords(ctx context.Context) ([]*entity.ChannelRecord, error) {
	return s.repo.ListRecords(ctx)
}

func newTradeNo() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "CH" + strconv.FormatInt(time.Now().UnixMilli(), 10) + hex.EncodeToString(buf)[:8], nil
}
