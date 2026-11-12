package service

import (
	"context"
	"fmt"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// HandlePaymentNotify 处理渠道支付通知：验签、匹配订单与金额、幂等记款、迟到支付转异常。
func (s *Service) HandlePaymentNotify(ctx context.Context, req NotifyInput) (bool, string) {
	now := time.Now().UTC()
	// 1. 验签
	if !s.verifySignature(req) {
		if _, err := s.recordNotifyAnomaly(ctx, req, "signature_invalid", "支付通知验签失败", now); err != nil {
			return false, err.Error()
		}
		return false, "signature invalid"
	}
	if req.Status != "SUCCESS" && req.Status != "FAILED" || req.ChannelTradeNo == "" {
		if _, err := s.recordNotifyAnomaly(ctx, req, "invalid_status", "通知状态或交易号非法", now); err != nil {
			return false, err.Error()
		}
		return false, "invalid payment status"
	}
	// 2. 匹配订单
	order, err := s.repo.FindByOrderNo(ctx, req.OrderNo)
	if err != nil {
		if _, err := s.recordNotifyAnomaly(ctx, req, "order_unknown", "支付通知无法匹配订单", now); err != nil {
			return false, err.Error()
		}
		return false, "order unknown"
	}
	// 3. 匹配金额
	if req.AmountCents != order.AmountCents() {
		if _, err := s.recordNotifyAnomaly(ctx, req, "amount_mismatch",
			fmt.Sprintf("通知金额 %d 与订单金额 %d 不符", req.AmountCents, order.AmountCents()), now); err != nil {
			return false, err.Error()
		}
		return false, "amount mismatch"
	}
	// 4. 重复通知幂等：同一渠道交易号已归属本订单时不重复记款
	if order.ChannelTradeNo == req.ChannelTradeNo && order.ChannelTradeNo != "" {
		if req.Status == "SUCCESS" && order.Status == entity.StatusPaid {
			if err := s.syncStock(ctx, order.OrderNo, "confirm"); err != nil {
				return false, err.Error()
			}
		}
		return true, "duplicate ignored"
	}
	if ex, err := s.repo.FindPayExceptionByTradeNo(ctx, req.ChannelTradeNo); err != nil {
		return false, err.Error()
	} else if ex != nil && ex.Reason == "late_payment" {
		if ex.OrderNo != req.OrderNo {
			return false, "trade no conflict"
		}
		return true, "duplicate ignored"
	}
	// 5. 支付失败通知：仅记流水，不改状态
	if req.Status == "FAILED" {
		if err := s.repo.CreateRecord(ctx, &entity.OrderRecord{
			OrderNo: order.OrderNo, Type: entity.RecordTypePay, Operator: "channel",
			Detail: fmt.Sprintf("渠道通知支付失败 trade_no=%s", req.ChannelTradeNo),
		}); err != nil {
			return false, err.Error()
		}
		return true, "failed notify recorded"
	}
	// 6. 有效期内支付成功：PENDING_PAYMENT -> PAID（条件更新含有效期判断，服务端时间）
	if order.Status == entity.StatusPendingPayment {
		changed, err := s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: order.OrderNo, Type: entity.RecordTypePay, Operator: "channel", Detail: fmt.Sprintf("支付成功 trade_no=%s amount=%d", req.ChannelTradeNo, req.AmountCents)}, func(r entity.Repository) (bool, error) {
			return r.MarkPaid(ctx, order.OrderNo, req.ChannelTradeNo, now)
		})
		if err != nil {
			if _, err := s.recordNotifyAnomaly(ctx, req, "trade_no_conflict", "渠道交易号已归属其他订单", now); err != nil {
				return false, err.Error()
			}
			return false, "trade no conflict"
		}
		if changed {
			if err := s.syncStock(ctx, order.OrderNo, "confirm"); err != nil {
				return false, err.Error()
			}
			return true, "paid"
		}
	}
	// 7. 已过期或已关闭：迟到支付，进入异常支付待处理列表（不复活订单、不重新占用库存）
	if order.Status == entity.StatusPendingPayment {
		// 通知到达时已过期：先按服务端时间关闭
		if _, err := s.closeExpired(ctx, order.OrderNo, now); err != nil {
			return false, err.Error()
		}
		order, err = s.repo.FindByOrderNo(ctx, order.OrderNo)
		if err != nil {
			return false, err.Error()
		}
	}
	if order.Status == entity.StatusCancelled || order.Status == entity.StatusClosedTimeout {
		if _, err := s.recordNotifyAnomaly(ctx, req, "late_payment", fmt.Sprintf("订单 %s 后收到支付成功通知，等待客服退款", statusText(order.Status)), now); err != nil {
			return false, err.Error()
		}
		return true, "late payment recorded"
	}
	// 8. 其他状态收到成功通知：异常记录，不改状态
	if _, err := s.recordNotifyAnomaly(ctx, req, "invalid_state",
		fmt.Sprintf("订单状态 %s 不应收支付成功通知", order.Status), now); err != nil {
		return false, err.Error()
	}
	return false, "invalid state"
}

// NotifyInput 是渠道发来的支付通知载荷。
type NotifyInput struct {
	OrderNo        string
	AmountCents    int64
	ChannelTradeNo string
	Status         string
	NotifiedAtUnix int64
	Signature      string
}

func (s *Service) verifySignature(req NotifyInput) bool {
	return VerifySignature(s.secret, req.OrderNo, req.AmountCents, req.ChannelTradeNo, req.Status, req.NotifiedAtUnix, req.Signature)
}

// recordNotifyAnomaly 登记异常支付记录（同一渠道交易号只登记一次）与订单流水。
func (s *Service) recordNotifyAnomaly(ctx context.Context, req NotifyInput, reason string, detail string, now time.Time) (bool, error) {
	if existing, err := s.repo.FindPayExceptionByTradeNo(ctx, req.ChannelTradeNo); err != nil {
		return false, err
	} else if existing != nil && !(reason == "late_payment" && existing.Reason != "late_payment") {
		err := s.repo.CreateRecord(ctx, &entity.OrderRecord{OrderNo: req.OrderNo, Type: entity.RecordTypeException, Operator: "channel", Detail: fmt.Sprintf("原始通知 reason=%s trade=%s status=%s amount=%d at=%d", reason, req.ChannelTradeNo, req.Status, req.AmountCents, req.NotifiedAtUnix)})
		return false, err
	}
	ex := &entity.PayException{
		OrderNo:        req.OrderNo,
		ChannelTradeNo: req.ChannelTradeNo,
		AmountCents:    req.AmountCents,
		Reason:         reason,
		Status:         entity.ExceptionPending,
		Detail:         detail,
	}
	return s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: req.OrderNo, Type: entity.RecordTypeException, Operator: "channel", Detail: fmt.Sprintf("异常支付 %s：trade_no=%s %s", reason, req.ChannelTradeNo, detail)}, func(r entity.Repository) (bool, error) {
		if err := r.CreatePayException(ctx, ex); err != nil {
			return false, err
		}
		return true, nil
	})
}

func statusText(status string) string {
	switch status {
	case entity.StatusCancelled:
		return "已取消"
	case entity.StatusClosedTimeout:
		return "超时关闭"
	default:
		return status
	}
}

// VerifySignature 校验通知签名：HMAC-SHA256(order_no|amount|trade_no|status|notified_at)。
func VerifySignature(secret string, orderNo string, amountCents int64, tradeNo string, status string, notifiedAt int64, signature string) bool {
	expected := SignPayload(secret, orderNo, amountCents, tradeNo, status, notifiedAt)
	return constantTimeEqual(expected, signature)
}
