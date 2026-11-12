package handler

import (
	"context"
	"net"
	"testing"

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

func TestAuditRegisteredProductRPCContract(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	productv1.RegisterProductServiceServer(server, newTestServer(t))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///product-test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := productv1.NewProductServiceClient(conn)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "ops01", "x-user-role", "ops"))
	created, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{Name: "真实RPC商品", PriceCents: 10000, StockTotal: 5})
	if err != nil {
		t.Fatalf("registered CreateProduct RPC: %v", err)
	}
	updated, err := client.UpdateProduct(ctx, &productv1.UpdateProductRequest{Id: created.Id, SetPrice: true, PriceCents: 9000})
	if err != nil || updated.PriceCents != 9000 {
		t.Fatalf("registered UpdateProduct RPC: %v / %+v", err, updated)
	}
	list, err := client.ListProducts(context.Background(), &productv1.ListProductsRequest{})
	if err != nil || len(list.Items) != 1 || list.Items[0].Id != created.Id {
		t.Fatalf("registered ListProducts RPC: %v / %+v", err, list)
	}
	if _, err := client.CreateProduct(context.Background(), &productv1.CreateProductRequest{Name: "无权", PriceCents: 1, StockTotal: 1}); err == nil {
		t.Fatal("anonymous RPC must not create products")
	}
}
