package handler

import (
	"context"
	"fmt"

	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/warehouse/service"
)

// Server 将 warehouse gRPC 请求适配到仓库用例。
type Server struct {
	warehousev1.UnimplementedWarehouseServiceServer
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

// ListPendingShip 代理订单服务的可发货订单查询。
func (s *Server) ListPendingShip(ctx context.Context, req *warehousev1.ListPendingShipRequest) (*warehousev1.ListPendingShipResponse, error) {
	orders, err := s.svc.ListPendingShip(ctx)
	if err != nil {
		return nil, err
	}
	resp := &warehousev1.ListPendingShipResponse{}
	for _, order := range orders {
		resp.Items = append(resp.Items, &warehousev1.PendingShipOrderDTO{
			OrderNo:     order.OrderNo,
			BuyerId:     order.BuyerID,
			ProductName: order.ProductName,
			Quantity:    order.Quantity,
			AmountCents: order.AmountCents,
			PaidAt:      order.PaidAt,
		})
	}
	return resp, nil
}

// Ship 代理订单服务的发货登记。
func (s *Server) Ship(ctx context.Context, req *warehousev1.ShipRequest) (*warehousev1.ShipResponse, error) {
	result, err := s.svc.Ship(ctx, req.GetOrderNo(), req.GetCarrier(), req.GetTrackingNo(), fmt.Sprintf("warehouse"))
	if err != nil {
		if result.Success {
			return nil, err
		}
		return &warehousev1.ShipResponse{Success: false, Message: result.Message}, nil
	}
	return &warehousev1.ShipResponse{Success: result.Success, Message: result.Message}, nil
}
