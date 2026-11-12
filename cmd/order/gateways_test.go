package main

import (
	"context"
	"errors"
	pay "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	p "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"google.golang.org/grpc"
	"testing"
)

type fakeProduct struct {
	p.ProductServiceClient
	err error
	ok  bool
}

func (f fakeProduct) ReserveStock(context.Context, *p.ReserveStockRequest, ...grpc.CallOption) (*p.ReserveStockResponse, error) {
	return &p.ReserveStockResponse{Ok: f.ok, Message: "denied"}, f.err
}
func (f fakeProduct) GetProduct(context.Context, *p.GetProductRequest, ...grpc.CallOption) (*p.ProductResponse, error) {
	return &p.ProductResponse{Name: "a", PriceCents: 99}, f.err
}

type fakePay struct {
	pay.PaymentServiceClient
	err error
	ok  bool
}

func (f fakePay) Pay(context.Context, *pay.PayRequest, ...grpc.CallOption) (*pay.PayResponse, error) {
	return &pay.PayResponse{Accepted: f.ok, ChannelTradeNo: "t", Message: "denied"}, f.err
}
func (f fakePay) Refund(context.Context, *pay.RefundRequest, ...grpc.CallOption) (*pay.RefundResponse, error) {
	return &pay.RefundResponse{Success: f.ok, Message: "m"}, f.err
}
func TestGateways(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		ok  bool
		err error
	}{{true, nil}, {false, nil}, {false, errors.New("offline")}} {
		stock := productStock{client: fakeProduct{ok: tc.ok, err: tc.err}}
		if e := stock.Reserve(ctx, "o", "p", 1); (e == nil) != (tc.ok && tc.err == nil) {
			t.Fatal(e)
		}
		n, a, e := stock.GetProduct(ctx, "p")
		if tc.err == nil && (n != "a" || a != 99 || e != nil) {
			t.Fatal(n, a, e)
		}
		if tc.err != nil && !errors.Is(e, tc.err) {
			t.Fatal(e)
		}
		c := channelGateway{client: fakePay{ok: tc.ok, err: tc.err}}
		_, e = c.Pay(ctx, "o", "b", 99)
		if (e == nil) != (tc.ok && tc.err == nil) {
			t.Fatal(e)
		}
		ok, msg, e := c.Refund(ctx, "r", "t", 99, "why")
		if tc.err == nil && (ok != tc.ok || msg != "m" || e != nil) {
			t.Fatal(ok, msg, e)
		}
	}
}
