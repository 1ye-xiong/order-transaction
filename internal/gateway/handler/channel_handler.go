package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// ChannelHandler 提供模拟支付渠道控制台的 HTTP 接口（教学模拟，非真实第三方）。
type ChannelHandler struct {
	client paymentv1.PaymentServiceClient
	log    *zap.Logger
}

func NewChannelHandler(client paymentv1.PaymentServiceClient, log *zap.Logger) *ChannelHandler {
	return &ChannelHandler{client: client, log: log}
}

// GetConfig 查看渠道当前行为配置。
func (h *ChannelHandler) GetConfig(c *gin.Context) {
	resp, err := h.client.GetChannelConfig(outgoingContext(c), &paymentv1.GetChannelConfigRequest{})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// SetConfig 更新渠道行为配置：success/fail/no_notify、重复通知、迟到通知、退款第 N 次起成功。
func (h *ChannelHandler) SetConfig(c *gin.Context) {
	var req request.ChannelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.SetChannelConfig(outgoingContext(c), &paymentv1.SetChannelConfigRequest{
		PayBehavior:              req.PayBehavior,
		NotifyCount:              req.NotifyCount,
		NotifyDelaySeconds:       req.NotifyDelaySeconds,
		PayLateSeconds:           req.PayLateSeconds,
		RefundSuccessFromAttempt: req.RefundSuccessFromAttempt,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Records 渠道流水。
func (h *ChannelHandler) Records(c *gin.Context) {
	resp, err := h.client.ListChannelRecords(outgoingContext(c), &paymentv1.ListChannelRecordsRequest{})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}
