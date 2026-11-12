package service

import (
	"context"
	"fmt"
)

// OrderClient 是订单服务的出站接口（发货相关）。
type OrderClient interface {
	ListShippable(ctx context.Context) ([]ShippableOrder, error)
	Ship(ctx context.Context, orderNo string, carrier string, trackingNo string, operator string) error
}

// ShippableOrder 是可发货订单的最小视图。
type ShippableOrder struct {
	OrderNo     string
	BuyerID     string
	ProductName string
	Quantity    int64
	AmountCents int64
	PaidAt      string
}

// ShipResult 发货登记结果。
type ShipResult struct {
	Success bool
	Message string
}

// Service 编排仓库用例：查询待发货、登记发货。发货合法性校验在订单服务内完成。
type Service struct {
	orders OrderClient
}

func NewService(orders OrderClient) *Service {
	return &Service{orders: orders}
}

// ListPendingShip 查询可发货订单列表。
func (s *Service) ListPendingShip(ctx context.Context) ([]ShippableOrder, error) {
	return s.orders.ListShippable(ctx)
}

// Ship 登记承运方与运单号；订单服务保证每单只发一次。
func (s *Service) Ship(ctx context.Context, orderNo string, carrier string, trackingNo string, operator string) (ShipResult, error) {
	if carrier == "" || trackingNo == "" {
		return ShipResult{Success: false, Message: "承运方与运单号必填"}, fmt.Errorf("carrier and tracking no required")
	}
	if err := s.orders.Ship(ctx, orderNo, carrier, trackingNo, operator); err != nil {
		return ShipResult{Success: false, Message: err.Error()}, nil
	}
	return ShipResult{Success: true, Message: "发货登记成功"}, nil
}
