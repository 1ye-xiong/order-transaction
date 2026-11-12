package handler

import (
	"context"
	"strconv"

	stdcodes "google.golang.org/grpc/codes"
	stdstatus "google.golang.org/grpc/status"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/order/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"go.uber.org/zap"
)

// Server 将 order gRPC 请求适配到订单用例。身份取自 gateway 透传的 metadata。
type Server struct {
	orderv1.UnimplementedOrderServiceServer
	svc *service.Service
	log *zap.Logger
}

func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

func requireBuyer(ctx context.Context) (string, error) {
	userID, role := grpcx.IdentityFromContext(ctx)
	if role != "buyer" || userID == "" {
		return "", apperrors.New(apperrors.KindForbidden, "forbidden", "only buyer can perform this operation")
	}
	return userID, nil
}

func requireRole(ctx context.Context, roles ...string) (string, error) {
	userID, role := grpcx.IdentityFromContext(ctx)
	for _, r := range roles {
		if role == r && userID != "" {
			return userID, nil
		}
	}
	return "", apperrors.New(apperrors.KindForbidden, "forbidden", "insufficient role")
}

func parseUint(text string) uint {
	value, _ := strconv.ParseUint(text, 10, 64)
	return uint(value)
}

// CreateOrder 买家下单。
func (s *Server) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.OrderResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.svc.CreateOrder(ctx, dto.CreateCommand{BuyerID: buyerID, ProductID: req.GetProductId(), Quantity: req.GetQuantity()})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// GetOrder 买家查询本人订单。
func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.OrderResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.svc.GetOrder(ctx, buyerID, req.GetOrderNo())
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// ListMyOrders 买家查询本人订单列表。
func (s *Server) ListMyOrders(ctx context.Context, req *orderv1.ListMyOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := s.svc.ListMyOrders(ctx, buyerID, req.GetPage(), req.GetPageSize())
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListOrdersResponse{Total: total}
	for _, item := range items {
		resp.Items = append(resp.Items, toOrderProto(item))
	}
	return resp, nil
}

// CancelOrder 买家取消待付款订单。
func (s *Server) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.OrderResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.svc.CancelOrder(ctx, buyerID, req.GetOrderNo(), "买家主动取消")
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// PayOrder 买家发起支付。
func (s *Server) PayOrder(ctx context.Context, req *orderv1.PayOrderRequest) (*orderv1.PayOrderResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	tradeNo, err := s.svc.PayOrder(ctx, buyerID, req.GetOrderNo())
	if err != nil {
		return nil, mapOrderError(err)
	}
	return &orderv1.PayOrderResponse{OrderNo: req.GetOrderNo(), ChannelTradeNo: tradeNo, Accepted: true, Message: "渠道受理，等待异步通知"}, nil
}

// ApplyRefund 买家申请售前退款。
func (s *Server) ApplyRefund(ctx context.Context, req *orderv1.ApplyRefundRequest) (*orderv1.OrderResponse, error) {
	buyerID, err := requireBuyer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.svc.ApplyRefund(ctx, dto.ApplyRefundCommand{BuyerID: buyerID, OrderNo: req.GetOrderNo(), Reason: req.GetReason()})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toOrderProto(item), nil
}

// HandlePaymentNotify 渠道回调，须验签并幂等处理。
func (s *Server) HandlePaymentNotify(ctx context.Context, req *orderv1.HandlePaymentNotifyRequest) (*orderv1.HandlePaymentNotifyResponse, error) {
	accepted, reason := s.svc.HandlePaymentNotify(ctx, service.NotifyInput{
		OrderNo:        req.GetOrderNo(),
		AmountCents:    req.GetAmountCents(),
		ChannelTradeNo: req.GetChannelTradeNo(),
		Status:         req.GetStatus(),
		NotifiedAtUnix: req.GetNotifiedAtUnix(),
		Signature:      req.GetSignature(),
	})
	return &orderv1.HandlePaymentNotifyResponse{Accepted: accepted, Reason: reason}, nil
}

func toOrderProto(item *dto.OrderDTO) *orderv1.OrderResponse {
	return &orderv1.OrderResponse{
		OrderNo:          item.OrderNo,
		BuyerId:          item.BuyerID,
		ProductId:        item.ProductID,
		ProductName:      item.ProductName,
		UnitPriceCents:   item.UnitPriceCents,
		Quantity:         item.Quantity,
		AmountCents:      item.AmountCents,
		Status:           item.Status,
		ChannelTradeNo:   item.ChannelTradeNo,
		PaidAt:           item.PaidAt,
		Carrier:          item.Carrier,
		TrackingNo:       item.TrackingNo,
		ShippedAt:        item.ShippedAt,
		CancelReason:     item.CancelReason,
		ClosedAt:         item.ClosedAt,
		RefundFailReason: item.RefundFailReason,
		RefundedAt:       item.RefundedAt,
		PayExpireAt:      item.PayExpireAt,
		CreatedAt:        item.CreatedAt,
	}
}

func toRefundProto(item *entity.RefundRequest) *orderv1.RefundRequestDTO {
	var reviewedAt string
	if item.ReviewedAt != nil {
		reviewedAt = item.ReviewedAt.Format("2006-01-02 15:04:05")
	}
	return &orderv1.RefundRequestDTO{
		Id:           strconv.FormatUint(uint64(item.ID), 10),
		OrderNo:      item.OrderNo,
		BuyerId:      item.BuyerID,
		Reason:       item.Reason,
		Status:       item.Status,
		ReviewerId:   item.ReviewerID,
		ReviewReason: item.ReviewReason,
		CreatedAt:    item.CreatedAt.Format("2006-01-02 15:04:05"),
		ReviewedAt:   reviewedAt,
	}
}

func toExceptionProto(item *entity.PayException) *orderv1.PayExceptionDTO {
	var handledAt string
	if item.HandledAt != nil {
		handledAt = item.HandledAt.Format("2006-01-02 15:04:05")
	}
	return &orderv1.PayExceptionDTO{
		Id:             strconv.FormatUint(uint64(item.ID), 10),
		OrderNo:        item.OrderNo,
		ChannelTradeNo: item.ChannelTradeNo,
		AmountCents:    item.AmountCents,
		Reason:         item.Reason,
		Status:         item.Status,
		Detail:         item.Detail,
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
		HandledAt:      handledAt,
	}
}

func mapOrderError(err error) error {
	// 下游服务（商品/渠道）的 gRPC 错误保留原始 kind 与提示
	if st, ok := stdstatus.FromError(err); ok && st.Code() != stdcodes.OK {
		return apperrors.FromGRPC(err)
	}
	switch err.Error() {
	case entity.ErrOrderNotFound.Error():
		return apperrors.NotFound("order_not_found", "order not found")
	case entity.ErrNotOrderOwner.Error():
		return apperrors.New(apperrors.KindForbidden, "not_order_owner", "cannot access orders of other buyers")
	case entity.ErrInvalidQuantity.Error():
		return apperrors.InvalidArgument("invalid_quantity", "quantity must be between 1 and 2")
	case entity.ErrReasonRequired.Error():
		return apperrors.InvalidArgument("reason_required", "reason is required")
	case entity.ErrTrackingNoRequired.Error():
		return apperrors.InvalidArgument("tracking_no_required", "carrier and tracking no are required")
	}
	return apperrors.New(apperrors.KindConflict, "order_conflict", err.Error())
}
