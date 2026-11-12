package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// SignPayload 生成签名（与渠道共享密钥的 HMAC-SHA256）。
func SignPayload(secret string, orderNo string, amountCents int64, tradeNo string, status string, notifiedAt int64) string {
	payload := orderNo + "|" + strconv.FormatInt(amountCents, 10) + "|" + tradeNo + "|" + status + "|" + strconv.FormatInt(notifiedAt, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeEqual(a string, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// ApplyRefund 买家对已付款未发货订单申请整单退款，申请期间暂停发货。
func (s *Service) ApplyRefund(ctx context.Context, cmd dto.ApplyRefundCommand) (*dto.OrderDTO, error) {
	if cmd.Reason == "" {
		return nil, entity.ErrReasonRequired
	}
	order, err := s.repo.FindByOrderNo(ctx, cmd.OrderNo)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != cmd.BuyerID {
		return nil, entity.ErrNotOrderOwner
	}
	if !order.CanApplyRefund() {
		return nil, fmt.Errorf("order not refundable in status %s", order.Status)
	}
	// 幂等：已有待审核申请时直接返回
	if pending, _ := s.repo.FindPendingRefundByOrder(ctx, cmd.OrderNo); pending != nil {
		return dto.FromOrder(order), nil
	}
	now := time.Now().UTC()
	changed, err := s.repo.Transition(ctx, cmd.OrderNo, entity.StatusPaid, entity.StatusRefundRequested, nil, now)
	if err != nil {
		return nil, err
	}
	if !changed {
		order, _ = s.repo.FindByOrderNo(ctx, cmd.OrderNo)
		return nil, fmt.Errorf("order not refundable in status %s", order.Status)
	}
	_ = s.repo.CreateRefundRequest(ctx, &entity.RefundRequest{
		OrderNo:   cmd.OrderNo,
		BuyerID:   cmd.BuyerID,
		Reason:    cmd.Reason,
		Status:    entity.RefundPending,
		CreatedAt: now,
	})
	_ = s.repo.CreateRecord(ctx, &entity.OrderRecord{
		OrderNo: cmd.OrderNo, Type: entity.RecordTypeRefund, Operator: cmd.BuyerID,
		Detail: "买家申请售前退款，发货暂停：" + cmd.Reason,
	})
	order, _ = s.repo.FindByOrderNo(ctx, cmd.OrderNo)
	return dto.FromOrder(order), nil
}

// ReviewRefund 客服审核退款申请：拒绝则恢复可发货；同意则发起渠道退款。
func (s *Service) ReviewRefund(ctx context.Context, cmd dto.ReviewRefundCommand) (*dto.OrderDTO, error) {
	req, err := s.repo.FindRefundRequest(ctx, cmd.RefundID)
	if err != nil {
		return nil, entity.ErrOrderNotFound
	}
	if req.Status != entity.RefundPending {
		return nil, fmt.Errorf("refund request already reviewed")
	}
	if !cmd.Approve && cmd.Reason == "" {
		return nil, entity.ErrReasonRequired
	}
	now := time.Now().UTC()
	order, err := s.repo.FindByOrderNo(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	if order.Status != entity.StatusRefundRequested {
		return nil, fmt.Errorf("order not in refund review status")
	}
	if !cmd.Approve {
		req.Status = entity.RefundRejected
		req.ReviewerID = cmd.ReviewerID
		req.ReviewReason = cmd.Reason
		req.ReviewedAt = &now
		if err := s.repo.UpdateRefundRequest(ctx, req); err != nil {
			return nil, err
		}
		if _, err := s.repo.Transition(ctx, req.OrderNo, entity.StatusRefundRequested, entity.StatusPaid, nil, now); err != nil {
			return nil, err
		}
		_ = s.repo.CreateRecord(ctx, &entity.OrderRecord{
			OrderNo: req.OrderNo, Type: entity.RecordTypeRefund, Operator: cmd.ReviewerID,
			Detail: "客服拒绝退款：" + cmd.Reason,
		})
		order, _ = s.repo.FindByOrderNo(ctx, req.OrderNo)
		return dto.FromOrder(order), nil
	}
	// 同意：先落审核记录，再走渠道退款
	req.Status = entity.RefundApproved
	req.ReviewerID = cmd.ReviewerID
	req.ReviewReason = cmd.Reason
	req.ReviewedAt = &now
	if err := s.repo.UpdateRefundRequest(ctx, req); err != nil {
		return nil, err
	}
	return s.executeRefund(ctx, req.OrderNo, fmt.Sprintf("RF%d", req.ID), cmd.ReviewerID, "客服同意退款："+cmd.Reason)
}

// RetryRefund 客服对退款失败的订单重试退款（渠道按退款单号累计尝试次数）。
func (s *Service) RetryRefund(ctx context.Context, operatorID string, orderNo string) (*dto.OrderDTO, error) {
	order, err := s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order.Status != entity.StatusRefundFailed {
		return nil, fmt.Errorf("order not in refund-failed status")
	}
	req, err := s.repo.FindLatestRefundByOrder(ctx, orderNo)
	if err != nil || req == nil {
		return nil, fmt.Errorf("refund request not found")
	}
	return s.executeRefund(ctx, orderNo, fmt.Sprintf("RF%d", req.ID), operatorID, "客服重试退款")
}

// executeRefund 调用渠道退款；成功则 REFUNDED 并恢复库存，失败则 REFUND_FAILED 待客服重试。
func (s *Service) executeRefund(ctx context.Context, orderNo string, refundNo string, operatorID string, action string) (*dto.OrderDTO, error) {
	order, err := s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	success, message, err := s.channel.Refund(ctx, refundNo, order.ChannelTradeNo, order.AmountCents(), action)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if success {
		changed, err := s.repo.Transition(ctx, orderNo, entity.StatusRefundRequested, entity.StatusRefunded, func(o *entity.Order) {
			refundedAt := now
			o.RefundedAt = &refundedAt
		}, now)
		if err != nil {
			return nil, err
		}
		if !changed {
			// 状态可能已是 REFUND_FAILED（重试退款），从 REFUND_FAILED 转移
			changed, err = s.repo.Transition(ctx, orderNo, entity.StatusRefundFailed, entity.StatusRefunded, func(o *entity.Order) {
				refundedAt := now
				o.RefundedAt = &refundedAt
			}, now)
			if err != nil {
				return nil, err
			}
			if !changed {
				order, _ = s.repo.FindByOrderNo(ctx, orderNo)
				return dto.FromOrder(order), nil
			}
		}
		// 正常售前退款恢复可售数量；重复恢复幂等（商品服务按占用记录状态判断）
		if err := s.stock.Restore(ctx, orderNo); err != nil {
			return nil, err
		}
		_ = s.repo.CreateRecord(ctx, &entity.OrderRecord{
			OrderNo: orderNo, Type: entity.RecordTypeRefund, Operator: operatorID,
			Detail: fmt.Sprintf("退款成功 refund_no=%s amount=%d，恢复可售数量", refundNo, order.AmountCents()),
		})	} else {
		changed, err := s.repo.Transition(ctx, orderNo, entity.StatusRefundRequested, entity.StatusRefundFailed, func(o *entity.Order) {
			o.RefundFailReason = message
		}, now)
		if err != nil {
			return nil, err
		}
		if changed {
			_ = s.repo.CreateRecord(ctx, &entity.OrderRecord{
				OrderNo: orderNo, Type: entity.RecordTypeRefund, Operator: operatorID,
				Detail: fmt.Sprintf("退款失败 refund_no=%s：%s，等待客服重试", refundNo, message),
			})
		}
	}
	order, _ = s.repo.FindByOrderNo(ctx, orderNo)
	return dto.FromOrder(order), nil
}
