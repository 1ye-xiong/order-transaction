package service

import (
	"context"
	"fmt"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// ListRefundRequests 客服查询退款申请（可按状态过滤）。
func (s *Service) ListRefundRequests(ctx context.Context, status string) ([]*entity.RefundRequest, error) {
	return s.repo.ListRefundRequests(ctx, status)
}

// ListPayExceptions 客服查询异常支付列表（可按状态过滤）。
func (s *Service) ListPayExceptions(ctx context.Context, status string) ([]*entity.PayException, error) {
	return s.repo.ListPayExceptions(ctx, status)
}

// RefundPayException 客服核实迟到支付异常后发起退款。
// 迟到支付订单已释放库存，退款成功不恢复数量（假设 A5），结果登记在异常记录上（假设 A4）。
func (s *Service) RefundPayException(ctx context.Context, operatorID string, exceptionID uint) (*entity.PayException, bool, string, error) {
	ex, err := s.repo.FindPayException(ctx, exceptionID)
	if err != nil {
		return nil, false, "", fmt.Errorf("pay exception not found")
	}
	if ex.Status != entity.ExceptionPending {
		return ex, false, "exception already handled: " + ex.Status, nil
	}
	if ex.Reason != "late_payment" {
		return nil, false, "", fmt.Errorf("only verified late payments can be refunded")
	}
	refundNo := fmt.Sprintf("RFE%d", ex.ID)
	success, message, err := s.channel.Refund(ctx, refundNo, ex.ChannelTradeNo, ex.AmountCents, "迟到支付退款")
	if err != nil {
		return ex, false, "", err
	}
	now := time.Now().UTC()
	if success {
		ex.Status = entity.ExceptionRefunded
		ex.HandledAt = &now
		ex.Detail = ex.Detail + "；退款成功 refund_no=" + refundNo
	} else {
		ex.Status = entity.ExceptionPending
		ex.Detail = ex.Detail + "；退款失败 refund_no=" + refundNo + "：" + message
	}
	ex.Detail = fmt.Sprintf("%s（操作人 %s）", ex.Detail, operatorID)
	_, err = s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: ex.OrderNo, Type: entity.RecordTypeException, Operator: operatorID, Detail: fmt.Sprintf("迟到支付退款 success=%t refund_no=%s：%s", success, refundNo, message)}, func(r entity.Repository) (bool, error) {
		if err := r.UpdatePayException(ctx, ex); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, false, "", err
	}
	return ex, success, message, nil
}
