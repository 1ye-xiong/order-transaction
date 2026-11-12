package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// OrderHandler 将订单 HTTP 接口适配到 order gRPC client。
type OrderHandler struct {
	client orderv1.OrderServiceClient
	log    *zap.Logger
}

func NewOrderHandler(client orderv1.OrderServiceClient, log *zap.Logger) *OrderHandler {
	return &OrderHandler{client: client, log: log}
}

// Create 买家下单。
func (h *OrderHandler) Create(c *gin.Context) {
	var req request.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.CreateOrder(outgoingContext(c), &orderv1.CreateOrderRequest{
		ProductId: req.ProductID,
		Quantity:  req.Quantity,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.Created(c, resp)
}

// Get 买家查询本人订单详情。
func (h *OrderHandler) Get(c *gin.Context) {
	resp, err := h.client.GetOrder(outgoingContext(c), &orderv1.GetOrderRequest{OrderNo: c.Param("orderNo")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// ListMy 买家查询本人订单列表。
func (h *OrderHandler) ListMy(c *gin.Context) {
	resp, err := h.client.ListMyOrders(outgoingContext(c), &orderv1.ListMyOrdersRequest{Page: 1, PageSize: 50})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Cancel 买家取消待付款订单。
func (h *OrderHandler) Cancel(c *gin.Context) {
	resp, err := h.client.CancelOrder(outgoingContext(c), &orderv1.CancelOrderRequest{OrderNo: c.Param("orderNo")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Pay 买家发起支付（渠道异步通知结果）。
func (h *OrderHandler) Pay(c *gin.Context) {
	resp, err := h.client.PayOrder(outgoingContext(c), &orderv1.PayOrderRequest{OrderNo: c.Param("orderNo")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// ApplyRefund 买家申请售前退款。
func (h *OrderHandler) ApplyRefund(c *gin.Context) {
	var req request.ApplyRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.ApplyRefund(outgoingContext(c), &orderv1.ApplyRefundRequest{
		OrderNo: c.Param("orderNo"),
		Reason:  req.Reason,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}
