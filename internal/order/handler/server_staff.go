package handler

import (
	"context"
	"fmt"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// ListShippableOrders 仓库查询可发货订单。
func (s *Server) ListShippableOrders(ctx context.Context, req *orderv1.ListShippableOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	if _, err := requireRole(ctx, "warehouse"); err != nil {
		return nil, err
	}
	items, err := s.svc.ListShippableOrders(ctx)
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListOrdersResponse{}
	for _, item := range items {
		resp.Items = append(resp.Items, toOrderProto(item))
	}
	return resp, nil
}

// ShipOrder 仓库发货登记（每单一次）。
func (s *Server) ShipOrder(ctx context.Context, req *orderv1.ShipOrderRequest) (*orderv1.OrderResponse, error) {
	operator, err := requireRole(ctx, "warehouse")
	if err != nil {
		return nil, err
	}
	item, err := s.svc.ShipOrder(ctx, dto.ShipCommand{Operator: operator, OrderNo: req.GetOrderNo(), Carrier: req.GetCarrier(), TrackingNo: req.GetTrackingNo()})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// ListRefundRequests 客服查询退款申请。
func (s *Server) ListRefundRequests(ctx context.Context, req *orderv1.ListRefundRequestsRequest) (*orderv1.ListRefundRequestsResponse, error) {
	if _, err := requireRole(ctx, "cs"); err != nil {
		return nil, err
	}
	items, err := s.svc.ListRefundRequests(ctx, req.GetStatus())
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListRefundRequestsResponse{}
	for _, item := range items {
		resp.Items = append(resp.Items, toRefundProto(item))
	}
	return resp, nil
}

// ReviewRefund 客服审核退款申请（同意或拒绝并写明原因）。
func (s *Server) ReviewRefund(ctx context.Context, req *orderv1.ReviewRefundRequest) (*orderv1.OrderResponse, error) {
	operator, err := requireRole(ctx, "cs")
	if err != nil {
		return nil, err
	}
	item, err := s.svc.ReviewRefund(ctx, dto.ReviewRefundCommand{ReviewerID: operator, RefundID: parseUint(req.GetRefundId()), Approve: req.GetApprove(), Reason: req.GetReason()})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// RetryRefund 客服对退款失败的订单重试退款。
func (s *Server) RetryRefund(ctx context.Context, req *orderv1.RetryRefundRequest) (*orderv1.OrderResponse, error) {
	operator, err := requireRole(ctx, "cs")
	if err != nil {
		return nil, err
	}
	item, err := s.svc.RetryRefund(ctx, operator, req.GetOrderNo())
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// ListPayExceptions 客服查询异常支付列表。
func (s *Server) ListPayExceptions(ctx context.Context, req *orderv1.ListPayExceptionsRequest) (*orderv1.ListPayExceptionsResponse, error) {
	if _, err := requireRole(ctx, "cs"); err != nil {
		return nil, err
	}
	items, err := s.svc.ListPayExceptions(ctx, req.GetStatus())
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListPayExceptionsResponse{}
	for _, item := range items {
		resp.Items = append(resp.Items, toExceptionProto(item))
	}
	return resp, nil
}

// RefundPayException 客服核实异常支付后发起退款。
func (s *Server) RefundPayException(ctx context.Context, req *orderv1.RefundPayExceptionRequest) (*orderv1.PayExceptionResponse, error) {
	operator, err := requireRole(ctx, "cs")
	if err != nil {
		return nil, err
	}
	ex, success, message, err := s.svc.RefundPayException(ctx, operator, parseUint(req.GetExceptionId()))
	if err != nil {
		return nil, mapOrderError(err)
	}
	return &orderv1.PayExceptionResponse{Exception: toExceptionProto(ex), Success: success, Message: message}, nil
}

// OpsSummary 运营核查汇总。
func (s *Server) OpsSummary(ctx context.Context, req *orderv1.OpsSummaryRequest) (*orderv1.OpsSummaryResponse, error) {
	if _, err := requireRole(ctx, "ops"); err != nil {
		return nil, err
	}
	paid, refunded, counts, exceptions, err := s.svc.OpsSummary(ctx)
	if err != nil {
		return nil, mapOrderError(err)
	}
	return &orderv1.OpsSummaryResponse{
		TotalPaidCents:     paid,
		TotalRefundedCents: refunded,
		PendingPayment:     counts[entity.StatusPendingPayment],
		Paid:               counts[entity.StatusPaid],
		Shipped:            counts[entity.StatusShipped],
		Cancelled:          counts[entity.StatusCancelled],
		ClosedTimeout:      counts[entity.StatusClosedTimeout],
		Refunded:           counts[entity.StatusRefunded],
		RefundPending:      counts[entity.StatusRefundRequested],
		RefundFailed:       counts[entity.StatusRefundFailed],
		PayExceptions:      exceptions,
	}, nil
}

// ListOrderRecords 运营按订单查流水。
func (s *Server) ListOrderRecords(ctx context.Context, req *orderv1.ListOrderRecordsRequest) (*orderv1.ListOrderRecordsResponse, error) {
	if _, err := requireRole(ctx, "ops"); err != nil {
		return nil, err
	}
	items, err := s.svc.ListOrderRecords(ctx, req.GetOrderNo())
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListOrderRecordsResponse{}
	for _, item := range items {
		resp.Items = append(resp.Items, &orderv1.OrderRecordDTO{
			Id:        fmt.Sprintf("%d", item.ID),
			OrderNo:   item.OrderNo,
			Type:      item.Type,
			Operator:  item.Operator,
			Detail:    item.Detail,
			CreatedAt: item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return resp, nil
}
