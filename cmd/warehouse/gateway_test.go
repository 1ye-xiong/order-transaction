package main

import (
	"context"
	"errors"
	o "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"google.golang.org/grpc"
	"testing"
)

type fakeOrders struct {
	o.OrderServiceClient
	err error
}

func (f fakeOrders) ListShippableOrders(context.Context, *o.ListShippableOrdersRequest, ...grpc.CallOption) (*o.ListOrdersResponse, error) {
	return &o.ListOrdersResponse{Items: []*o.OrderResponse{{OrderNo: "o", BuyerId: "b", ProductName: "p", Quantity: 1, AmountCents: 99, PaidAt: "2026-11-12"}}}, f.err
}
func (f fakeOrders) ShipOrder(context.Context, *o.ShipOrderRequest, ...grpc.CallOption) (*o.OrderResponse, error) {
	return &o.OrderResponse{}, f.err
}
func TestOrderGateway(t *testing.T) {
	for _, err := range []error{nil, errors.New("offline")} {
		g := orderGateway{client: fakeOrders{err: err}}
		items, e := g.ListShippable(context.Background())
		if err == nil && (e != nil || len(items) != 1 || items[0].AmountCents != 99) {
			t.Fatal(items, e)
		}
		if err != nil && !errors.Is(e, err) {
			t.Fatal(e)
		}
		e = g.Ship(context.Background(), "o", "c", "t", "u")
		if (e == nil) != (err == nil) {
			t.Fatal(e)
		}
	}
}
