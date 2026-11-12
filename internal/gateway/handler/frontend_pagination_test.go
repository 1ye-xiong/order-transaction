package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type paginationCaptureClient struct {
	orderv1.OrderServiceClient
	last  *orderv1.ListMyOrdersRequest
	calls int
}

func (c *paginationCaptureClient) ListMyOrders(_ context.Context, r *orderv1.ListMyOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	c.last = r
	c.calls++
	return &orderv1.ListOrdersResponse{Total: 21, Items: []*orderv1.OrderResponse{{OrderNo: "SECOND-PAGE"}}}, nil
}
func TestFrontendPaginationGatewayContract(t *testing.T) {
	client := &paginationCaptureClient{}
	r := gin.New()
	r.GET("/mine", NewOrderHandler(client, zap.NewNop()).ListMy)
	for _, query := range []string{"?page=2&page_size=20", ""} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", "/mine"+query, nil))
		if rec.Code != 200 {
			t.Fatalf("valid pagination rejected: %d %s", rec.Code, rec.Body.String())
		}
		page, size := int32(2), int32(20)
		if query == "" {
			page, size = 1, 50
		}
		if client.last.Page != page || client.last.PageSize != size {
			t.Fatalf("pagination not forwarded: %+v", client.last)
		}
	}
	calls := client.calls
	for _, query := range []string{"?page=0", "?page=-1", "?page=abc", "?page=2147483648", "?page_size=0", "?page_size=101", "?page_size=invalid"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", "/mine"+query, nil))
		if rec.Code != 400 {
			t.Fatalf("invalid pagination accepted %s: %d", query, rec.Code)
		}
	}
	if client.calls != calls {
		t.Fatal("invalid query reached downstream")
	}
}
