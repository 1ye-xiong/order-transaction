package handler

import (
	"context"
	"strconv"

	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/payment/service"
	"go.uber.org/zap"
)

// Server 将 payment gRPC 请求适配到模拟渠道用例。
type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
	svc *service.Service
	log *zap.Logger
}

func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// Pay 受理付款并按配置异步发送通知。
func (s *Server) Pay(ctx context.Context, req *paymentv1.PayRequest) (*paymentv1.PayResponse, error) {
	tradeNo, err := s.svc.Pay(ctx, req.GetOrderNo(), req.GetBuyerId(), req.GetAmountCents())
	if err != nil {
		return &paymentv1.PayResponse{Accepted: false, Message: err.Error()}, nil
	}
	return &paymentv1.PayResponse{Accepted: true, ChannelTradeNo: tradeNo, Message: "accepted"}, nil
}

// Refund 受理退款并返回即时结果。
func (s *Server) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	success, message, err := s.svc.Refund(ctx, req.GetRefundNo(), req.GetChannelTradeNo(), req.GetAmountCents(), req.GetReason())
	if err != nil {
		return &paymentv1.RefundResponse{Success: false, Message: err.Error()}, nil
	}
	return &paymentv1.RefundResponse{Success: success, Message: message}, nil
}

// SetChannelConfig 更新渠道行为配置（教学控制台）。
func (s *Server) SetChannelConfig(ctx context.Context, req *paymentv1.SetChannelConfigRequest) (*paymentv1.ChannelConfigResponse, error) {
	config := entity.ChannelConfig{
		PayBehavior:              req.GetPayBehavior(),
		NotifyCount:              int(req.GetNotifyCount()),
		NotifyDelaySeconds:       int(req.GetNotifyDelaySeconds()),
		PayLateSeconds:           int(req.GetPayLateSeconds()),
		RefundSuccessFromAttempt: int(req.GetRefundSuccessFromAttempt()),
	}
	if err := s.svc.SetConfig(ctx, config); err != nil {
		return nil, err
	}
	return toConfigResponse(req.GetPayBehavior(), int(req.GetNotifyCount()), int(req.GetNotifyDelaySeconds()), int(req.GetPayLateSeconds()), int(req.GetRefundSuccessFromAttempt())), nil
}

// GetChannelConfig 查看当前渠道行为配置。
func (s *Server) GetChannelConfig(ctx context.Context, req *paymentv1.GetChannelConfigRequest) (*paymentv1.ChannelConfigResponse, error) {
	config, err := s.svc.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	return toConfigResponse(config.PayBehavior, config.NotifyCount, config.NotifyDelaySeconds, config.PayLateSeconds, config.RefundSuccessFromAttempt), nil
}

// ListChannelRecords 渠道流水。
func (s *Server) ListChannelRecords(ctx context.Context, req *paymentv1.ListChannelRecordsRequest) (*paymentv1.ListChannelRecordsResponse, error) {
	records, err := s.svc.ListRecords(ctx)
	if err != nil {
		return nil, err
	}
	resp := &paymentv1.ListChannelRecordsResponse{}
	for _, record := range records {
		resp.Items = append(resp.Items, &paymentv1.ChannelRecordDTO{
			Id:             strconv.FormatUint(uint64(record.ID), 10),
			Type:           record.Type,
			ChannelTradeNo: record.ChannelTradeNo,
			RefundNo:       record.RefundNo,
			OrderNo:        record.OrderNo,
			AmountCents:    record.AmountCents,
			Result:         record.Result,
			Attempts:       int32(record.Attempts),
			Detail:         record.Detail,
			CreatedAt:      record.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return resp, nil
}

func toConfigResponse(behavior string, notifyCount int, delay int, late int, refundFrom int) *paymentv1.ChannelConfigResponse {
	return &paymentv1.ChannelConfigResponse{
		PayBehavior:              behavior,
		NotifyCount:              int32(notifyCount),
		NotifyDelaySeconds:       int32(delay),
		PayLateSeconds:           int32(late),
		RefundSuccessFromAttempt: int32(refundFrom),
	}
}
