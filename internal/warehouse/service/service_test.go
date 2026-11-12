package service

import (
	"context"
	"errors"
	"testing"
)

type fakeOrders struct {
	listErr   error
	shipErr   error
	shipCalls int
	listCalls int
	orders    []ShippableOrder
}

func (f *fakeOrders) ListShippable(_ context.Context) ([]ShippableOrder, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.orders, nil
}

func (f *fakeOrders) Ship(_ context.Context, _ string, _ string, _ string, _ string) error {
	f.shipCalls++
	return f.shipErr
}

func TestListPendingShip(t *testing.T) {
	orders := &fakeOrders{orders: []ShippableOrder{{OrderNo: "ORD-1", ProductName: "商品", Quantity: 1, AmountCents: 10000}}}
	s := NewService(orders)
	items, err := s.ListPendingShip(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %v, err = %v", items, err)
	}
	if items[0].OrderNo != "ORD-1" {
		t.Fatalf("orderNo = %s", items[0].OrderNo)
	}
}

func TestListPendingShipError(t *testing.T) {
	s := NewService(&fakeOrders{listErr: errors.New("boom")})
	if _, err := s.ListPendingShip(context.Background()); err == nil {
		t.Fatal("应透传下游错误")
	}
}

// 发货登记：订单服务校验合法性，仓库只透传结果。
func TestShip(t *testing.T) {
	orders := &fakeOrders{}
	s := NewService(orders)
	result, err := s.Ship(context.Background(), "ORD-1", "顺丰", "SF001", "warehouse01")
	if err != nil || !result.Success {
		t.Fatalf("ship: %v / %+v", err, result)
	}
	if orders.shipCalls != 1 {
		t.Fatalf("shipCalls = %d", orders.shipCalls)
	}

	// 参数缺失
	result, err = s.Ship(context.Background(), "ORD-1", "", "", "warehouse01")
	if err == nil || result.Success {
		t.Fatal("缺少承运方/运单号应失败")
	}
	if orders.shipCalls != 1 {
		t.Fatalf("参数缺失不应调用下游, shipCalls = %d", orders.shipCalls)
	}

	// 下游拒绝（如退款申请中）
	orders.shipErr = errors.New("refund requested: shipping paused")
	result, err = s.Ship(context.Background(), "ORD-2", "顺丰", "SF002", "warehouse01")
	if err != nil || result.Success {
		t.Fatalf("下游拒绝应返回受限结果: %v / %+v", err, result)
	}
	if result.Message == "" {
		t.Fatal("受限结果应包含原因")
	}
}
