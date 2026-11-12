package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// StaffHandler 聚合仓库、客服、运营相关的 HTTP 接口。
type StaffHandler struct {
	order     orderv1.OrderServiceClient
	warehouse warehousev1.WarehouseServiceClient
	log       *zap.Logger
}

func NewStaffHandler(order orderv1.OrderServiceClient, warehouse warehousev1.WarehouseServiceClient, log *zap.Logger) *StaffHandler {
	return &StaffHandler{order: order, warehouse: warehouse, log: log}
}

// PendingShip 仓库查询可发货订单。
func (h *StaffHandler) PendingShip(c *gin.Context) {
	resp, err := h.warehouse.ListPendingShip(outgoingContext(c), &warehousev1.ListPendingShipRequest{})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Ship 仓库发货登记。
func (h *StaffHandler) Ship(c *gin.Context) {
	var req request.ShipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.warehouse.Ship(outgoingContext(c), &warehousev1.ShipRequest{
		OrderNo:    c.Param("orderNo"),
		Carrier:    req.Carrier,
		TrackingNo: req.TrackingNo,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// RefundRequests 客服查询退款申请（可按状态过滤）。
func (h *StaffHandler) RefundRequests(c *gin.Context) {
	resp, err := h.order.ListRefundRequests(outgoingContext(c), &orderv1.ListRefundRequestsRequest{Status: c.Query("status")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// ReviewRefund 客服审核退款申请。
func (h *StaffHandler) ReviewRefund(c *gin.Context) {
	var req request.ReviewRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.order.ReviewRefund(outgoingContext(c), &orderv1.ReviewRefundRequest{
		RefundId: c.Param("refundId"),
		Approve:  req.Approve,
		Reason:   req.Reason,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// RetryRefund 客服重试退款失败的订单。
func (h *StaffHandler) RetryRefund(c *gin.Context) {
	resp, err := h.order.RetryRefund(outgoingContext(c), &orderv1.RetryRefundRequest{OrderNo: c.Param("orderNo")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// PayExceptions 客服查询异常支付列表。
func (h *StaffHandler) PayExceptions(c *gin.Context) {
	resp, err := h.order.ListPayExceptions(outgoingContext(c), &orderv1.ListPayExceptionsRequest{Status: c.Query("status")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// RefundPayException 客服对异常支付发起退款。
func (h *StaffHandler) RefundPayException(c *gin.Context) {
	resp, err := h.order.RefundPayException(outgoingContext(c), &orderv1.RefundPayExceptionRequest{ExceptionId: c.Param("exceptionId")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// OpsSummary 运营交易汇总。
func (h *StaffHandler) OpsSummary(c *gin.Context) {
	resp, err := h.order.OpsSummary(outgoingContext(c), &orderv1.OpsSummaryRequest{})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// OrderRecords 运营按订单核查流水。
func (h *StaffHandler) OrderRecords(c *gin.Context) {
	resp, err := h.order.ListOrderRecords(outgoingContext(c), &orderv1.ListOrderRecordsRequest{OrderNo: c.Param("orderNo")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}
