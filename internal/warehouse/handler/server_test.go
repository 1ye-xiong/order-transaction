package handler

import (
	"context"
	"errors"
	"testing"

	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/warehouse/service"
)

type fakeOrderClient struct {
	orders      []service.ShippableOrder
	listErr     error
	shipErr     error
	shipped     []string
	lastCarrier string
	lastTrack   string
	lastOp      string
}

func (f *fakeOrderClient) ListShippable(context.Context) ([]service.ShippableOrder, error) {
	return f.orders, f.listErr
}

func (f *fakeOrderClient) Ship(_ context.Context, orderNo string, carrier string, trackingNo string, operator string) error {
	f.shipped = append(f.shipped, orderNo)
	f.lastCarrier = carrier
	f.lastTrack = trackingNo
	f.lastOp = operator
	return f.shipErr
}

func TestListPendingShip(t *testing.T) {
	client := &fakeOrderClient{orders: []service.ShippableOrder{{
		OrderNo: "O-1", BuyerID: "buyer1", ProductName: "商品", Quantity: 2, AmountCents: 20000, PaidAt: "2026-04-28 10:00:00",
	}}}
	s := NewServer(service.NewService(client))

	resp, err := s.ListPendingShip(context.Background(), &warehousev1.ListPendingShipRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items = %d", len(resp.Items))
	}
	item := resp.Items[0]
	if item.OrderNo != "O-1" || item.BuyerId != "buyer1" || item.ProductName != "商品" ||
		item.Quantity != 2 || item.AmountCents != 20000 || item.PaidAt != "2026-04-28 10:00:00" {
		t.Fatalf("item = %+v", item)
	}

	failing := NewServer(service.NewService(&fakeOrderClient{listErr: errors.New("order service down")}))
	if _, err := failing.ListPendingShip(context.Background(), &warehousev1.ListPendingShipRequest{}); err == nil {
		t.Fatal("上游失败应返回错误")
	}
}

func TestShip(t *testing.T) {
	client := &fakeOrderClient{}
	s := NewServer(service.NewService(client))

	resp, err := s.Ship(context.Background(), &warehousev1.ShipRequest{OrderNo: "O-1", Carrier: "SF", TrackingNo: "SF001"})
	if err != nil || !resp.Success {
		t.Fatalf("ship: %v / %+v", err, resp)
	}
	if len(client.shipped) != 1 || client.shipped[0] != "O-1" ||
		client.lastCarrier != "SF" || client.lastTrack != "SF001" || client.lastOp != "warehouse" {
		t.Fatalf("上游调用 = %+v", client)
	}

	missing, err := s.Ship(context.Background(), &warehousev1.ShipRequest{OrderNo: "O-2"})
	if err != nil {
		t.Fatalf("缺参应返回业务失败而非错误: %v", err)
	}
	if missing.Success || missing.Message == "" {
		t.Fatalf("缺参响应 = %+v", missing)
	}

	rejected := NewServer(service.NewService(&fakeOrderClient{shipErr: errors.New("order not shippable")}))
	bad, err := rejected.Ship(context.Background(), &warehousev1.ShipRequest{OrderNo: "O-3", Carrier: "SF", TrackingNo: "SF003"})
	if err != nil {
		t.Fatalf("上游业务失败不应返回错误: %v", err)
	}
	if bad.Success || bad.Message != "order not shippable" {
		t.Fatalf("上游失败响应 = %+v", bad)
	}
}
