package handler

import (
	"context"
	stderrors "errors"

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/product/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"go.uber.org/zap"
)

// Server 将 product gRPC 请求适配到商品用例。
type Server struct {
	productv1.UnimplementedProductServiceServer
	svc *service.Service
	log *zap.Logger
}

func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// Create 创建商品，仅运营。
func (s *Server) Create(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.ProductResponse, error) {
	if err := requireRole(ctx, "ops"); err != nil {
		return nil, err
	}
	item, err := s.svc.Create(ctx, dto.CreateCommand{
		Name:       req.GetName(),
		PriceCents: req.GetPriceCents(),
		StockTotal: req.GetStockTotal(),
	})
	if err != nil {
		return nil, mapProductError(err)
	}
	return toProto(item), nil
}

// Update 调整售价/上下架/补货，仅运营。
func (s *Server) Update(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.ProductResponse, error) {
	if err := requireRole(ctx, "ops"); err != nil {
		return nil, err
	}
	item, err := s.svc.Update(ctx, dto.UpdateCommand{
		ID:         req.GetId(),
		SetPrice:   req.GetSetPrice(),
		PriceCents: req.GetPriceCents(),
		SetOnSale:  req.GetSetOnSale(),
		OnSale:     req.GetOnSale(),
		StockDelta: req.GetStockDelta(),
	})
	if err != nil {
		return nil, mapProductError(err)
	}
	return toProto(item), nil
}

// List 商品列表。
func (s *Server) List(ctx context.Context, req *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
	items, err := s.svc.List(ctx, req.GetIncludeOffSale())
	if err != nil {
		return nil, mapProductError(err)
	}
	resp := &productv1.ListProductsResponse{}
	for _, item := range items {
		resp.Items = append(resp.Items, toProto(item))
	}
	return resp, nil
}

// GetProduct 查询单个商品（订单服务下单前调用）。
func (s *Server) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.ProductResponse, error) {
	item, err := s.svc.GetProduct(ctx, req.GetId())
	if err != nil {
		return nil, mapProductError(err)
	}
	return toProto(item), nil
}

// ReserveStock 为订单占用库存（订单服务内部调用）。
func (s *Server) ReserveStock(ctx context.Context, req *productv1.ReserveStockRequest) (*productv1.ReserveStockResponse, error) {
	err := s.svc.Reserve(ctx, req.GetOrderNo(), req.GetProductId(), req.GetQuantity())
	if err != nil {
		return &productv1.ReserveStockResponse{Ok: false, Message: mapProductError(err).Error()}, nil
	}
	return &productv1.ReserveStockResponse{Ok: true}, nil
}

// ReleaseStock 释放订单占用（幂等）。
func (s *Server) ReleaseStock(ctx context.Context, req *productv1.ReleaseStockRequest) (*productv1.ReleaseStockResponse, error) {
	if err := s.svc.Release(ctx, req.GetOrderNo()); err != nil {
		return &productv1.ReleaseStockResponse{Ok: false}, nil
	}
	return &productv1.ReleaseStockResponse{Ok: true}, nil
}

// ConfirmStock 支付成功后确认扣减（幂等）。
func (s *Server) ConfirmStock(ctx context.Context, req *productv1.ConfirmStockRequest) (*productv1.ConfirmStockResponse, error) {
	if err := s.svc.Confirm(ctx, req.GetOrderNo()); err != nil {
		return &productv1.ConfirmStockResponse{Ok: false}, nil
	}
	return &productv1.ConfirmStockResponse{Ok: true}, nil
}

// RestoreStock 售前退款成功后恢复库存（幂等）。
func (s *Server) RestoreStock(ctx context.Context, req *productv1.RestoreStockRequest) (*productv1.RestoreStockResponse, error) {
	if err := s.svc.Restore(ctx, req.GetOrderNo()); err != nil {
		return &productv1.RestoreStockResponse{Ok: false}, nil
	}
	return &productv1.RestoreStockResponse{Ok: true}, nil
}

func requireRole(ctx context.Context, role string) error {
	_, userRole := grpcx.IdentityFromContext(ctx)
	if userRole != role {
		return apperrors.New(apperrors.KindForbidden, "forbidden", "only "+role+" can perform this operation")
	}
	return nil
}

func toProto(item *dto.ProductDTO) *productv1.ProductResponse {
	return &productv1.ProductResponse{
		Id:         item.ID,
		Name:       item.Name,
		PriceCents: item.PriceCents,
		StockTotal: item.StockTotal,
		Reserved:   item.Reserved,
		Available:  item.Available,
		OnSale:     item.OnSale,
		UpdatedAt:  item.UpdatedAt,
	}
}

func mapProductError(err error) error {
	switch {
	case stderrors.Is(err, entity.ErrProductNotFound):
		return apperrors.NotFound("product_not_found", "product not found")
	case stderrors.Is(err, entity.ErrOffSale):
		return apperrors.New(apperrors.KindConflict, "product_off_sale", "product off sale")
	case stderrors.Is(err, entity.ErrInsufficientStock):
		return apperrors.New(apperrors.KindConflict, "insufficient_stock", "insufficient available stock")
	case stderrors.Is(err, entity.ErrInvalidQuantity):
		return apperrors.InvalidArgument("invalid_quantity", "quantity must be between 1 and 2")
	case stderrors.Is(err, entity.ErrInvalidProduct):
		return apperrors.InvalidArgument("invalid_product", "invalid product input")
	case stderrors.Is(err, entity.ErrReservationMissing):
		return apperrors.NotFound("reservation_not_found", "stock reservation not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "product_service_error", "product service error", err)
	}
}
