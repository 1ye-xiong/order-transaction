package main

import (
	"context"
	"errors"
	"testing"

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"google.golang.org/grpc"
)

type auditProductClient struct {
	productv1.ProductServiceClient
	ok  bool
	err error
}

func (c auditProductClient) ReleaseStock(context.Context, *productv1.ReleaseStockRequest, ...grpc.CallOption) (*productv1.ReleaseStockResponse, error) {
	return &productv1.ReleaseStockResponse{Ok: c.ok}, c.err
}
func (c auditProductClient) ConfirmStock(context.Context, *productv1.ConfirmStockRequest, ...grpc.CallOption) (*productv1.ConfirmStockResponse, error) {
	return &productv1.ConfirmStockResponse{Ok: c.ok}, c.err
}
func (c auditProductClient) RestoreStock(context.Context, *productv1.RestoreStockRequest, ...grpc.CallOption) (*productv1.RestoreStockResponse, error) {
	return &productv1.RestoreStockResponse{Ok: c.ok}, c.err
}

func TestAuditStockClientRejectsBusinessFailure(t *testing.T) {
	for _, mode := range []string{"ok", "business failure", "transport failure"} {
		t.Run(mode, func(t *testing.T) {
			client := auditProductClient{ok: mode == "ok"}
			if mode == "transport failure" {
				client.err = errors.New("connection lost")
			}
			stock := &productStock{client: client}
			for name, call := range map[string]func(context.Context, string) error{"release": stock.Release, "confirm": stock.Confirm, "restore": stock.Restore} {
				err := call(context.Background(), "ORD-1")
				if (err == nil) != (mode == "ok") {
					t.Fatalf("%s returned %v for %s", name, err, mode)
				}
				if client.err != nil && !errors.Is(err, client.err) {
					t.Fatalf("%s lost transport error: %v", name, err)
				}
			}
		})
	}
}
