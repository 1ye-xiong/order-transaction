package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	paymentv1 "github.com/BwCloudWeGo/bw-cli/api/gen/payment/v1"
	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	warehousev1 "github.com/BwCloudWeGo/bw-cli/api/gen/warehouse/v1"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

var gwJWT = middleware.JWTConfig{Secret: "gw-test-secret", Issuer: "test", ExpireSeconds: 600}

func gwToken(t *testing.T, claims middleware.JWTClaims) string {
	t.Helper()
	token, err := middleware.GenerateToken(gwJWT, claims)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

// newTestGateway 构建带 JWT 中间件的网关测试路由。
func newTestGateway(t *testing.T, order *fakeOrderClient, product *fakeProductClient, payment *fakePaymentClient,
	warehouse *fakeWarehouseClient, user *fakeUserClient, note *fakeNoteClient) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("request_id", "req-1"); c.Next() })
	auth := middleware.JWTAuth(gwJWT)

	products := r.Group("/api/v1/products")
	{
		products.GET("", auth, NewProductHandler(product, zap.NewNop()).List)
		products.POST("", auth, NewProductHandler(product, zap.NewNop()).Create)
		products.PUT("/:id", auth, NewProductHandler(product, zap.NewNop()).Update)
	}
	orders := r.Group("/api/v1/orders")
	{
		oh := NewOrderHandler(order, zap.NewNop())
		orders.POST("", auth, oh.Create)
		orders.GET("/mine", auth, oh.ListMy)
		orders.GET("/:orderNo", auth, oh.Get)
		orders.POST("/:orderNo/cancel", auth, oh.Cancel)
		orders.POST("/:orderNo/pay", auth, oh.Pay)
		orders.POST("/:orderNo/refund", auth, oh.ApplyRefund)
	}
	staff := NewStaffHandler(order, warehouse, zap.NewNop())
	warehouseGroup := r.Group("/api/v1/warehouse")
	{
		warehouseGroup.GET("/orders", auth, staff.PendingShip)
		warehouseGroup.POST("/orders/:orderNo/ship", auth, staff.Ship)
	}
	cs := r.Group("/api/v1/cs")
	{
		cs.GET("/refunds", auth, staff.RefundRequests)
		cs.POST("/refunds/:refundId/review", auth, staff.ReviewRefund)
		cs.POST("/orders/:orderNo/refund-retry", auth, staff.RetryRefund)
		cs.GET("/pay-exceptions", auth, staff.PayExceptions)
		cs.POST("/pay-exceptions/:exceptionId/refund", auth, staff.RefundPayException)
	}
	ops := r.Group("/api/v1/ops")
	{
		ops.GET("/summary", auth, staff.OpsSummary)
		ops.GET("/orders/:orderNo/records", auth, staff.OrderRecords)
	}
	channel := r.Group("/api/v1/channel")
	{
		ch := NewChannelHandler(payment, zap.NewNop())
		channel.GET("/config", ch.GetConfig)
		channel.PUT("/config", ch.SetConfig)
		channel.GET("/records", ch.Records)
	}
	userHandler := NewUserHandler(user, gwJWT, zap.NewNop())
	users := r.Group("/api/v1/users")
	{
		users.POST("/register", userHandler.Register)
		users.POST("/login", userHandler.Login)
		users.GET("/me", auth, userHandler.CurrentUser)
		users.GET("/:id", auth, userHandler.GetUser)
	}
	notes := r.Group("/api/v1/notes")
	{
		nh := NewNoteHandler(note, zap.NewNop())
		notes.POST("", nh.Create)
		notes.GET("/:id", nh.Get)
		notes.POST("/publish", nh.PublishNote)
	}
	return r
}

func gwDo(t *testing.T, r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func gwDecode(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v / %s", err, w.Body.String())
	}
	return payload
}

// ---------- 测试替身 ----------

type fakeOrderClient struct{ err error }

func (f *fakeOrderClient) CreateOrder(context.Context, *orderv1.CreateOrderRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{OrderNo: "ORD-1", Status: "PENDING_PAYMENT"}, f.err
}
func (f *fakeOrderClient) GetOrder(context.Context, *orderv1.GetOrderRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{OrderNo: "ORD-1", Status: "PAID"}, f.err
}
func (f *fakeOrderClient) ListMyOrders(context.Context, *orderv1.ListMyOrdersRequest, ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return &orderv1.ListOrdersResponse{Total: 1}, f.err
}
func (f *fakeOrderClient) CancelOrder(context.Context, *orderv1.CancelOrderRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{OrderNo: "ORD-1", Status: "CANCELLED"}, f.err
}
func (f *fakeOrderClient) PayOrder(context.Context, *orderv1.PayOrderRequest, ...grpc.CallOption) (*orderv1.PayOrderResponse, error) {
	return &orderv1.PayOrderResponse{Accepted: true}, f.err
}
func (f *fakeOrderClient) ApplyRefund(context.Context, *orderv1.ApplyRefundRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{OrderNo: "ORD-1", Status: "REFUND_REQUESTED"}, f.err
}
func (f *fakeOrderClient) HandlePaymentNotify(context.Context, *orderv1.HandlePaymentNotifyRequest, ...grpc.CallOption) (*orderv1.HandlePaymentNotifyResponse, error) {
	return &orderv1.HandlePaymentNotifyResponse{Accepted: true}, f.err
}
func (f *fakeOrderClient) ListShippableOrders(context.Context, *orderv1.ListShippableOrdersRequest, ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return &orderv1.ListOrdersResponse{}, f.err
}
func (f *fakeOrderClient) ShipOrder(context.Context, *orderv1.ShipOrderRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{Status: "SHIPPED"}, f.err
}
func (f *fakeOrderClient) ListRefundRequests(context.Context, *orderv1.ListRefundRequestsRequest, ...grpc.CallOption) (*orderv1.ListRefundRequestsResponse, error) {
	return &orderv1.ListRefundRequestsResponse{}, f.err
}
func (f *fakeOrderClient) ReviewRefund(context.Context, *orderv1.ReviewRefundRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{Status: "REFUNDED"}, f.err
}
func (f *fakeOrderClient) RetryRefund(context.Context, *orderv1.RetryRefundRequest, ...grpc.CallOption) (*orderv1.OrderResponse, error) {
	return &orderv1.OrderResponse{Status: "REFUNDED"}, f.err
}
func (f *fakeOrderClient) ListPayExceptions(context.Context, *orderv1.ListPayExceptionsRequest, ...grpc.CallOption) (*orderv1.ListPayExceptionsResponse, error) {
	return &orderv1.ListPayExceptionsResponse{}, f.err
}
func (f *fakeOrderClient) RefundPayException(context.Context, *orderv1.RefundPayExceptionRequest, ...grpc.CallOption) (*orderv1.PayExceptionResponse, error) {
	return &orderv1.PayExceptionResponse{Success: true}, f.err
}
func (f *fakeOrderClient) OpsSummary(context.Context, *orderv1.OpsSummaryRequest, ...grpc.CallOption) (*orderv1.OpsSummaryResponse, error) {
	return &orderv1.OpsSummaryResponse{TotalPaidCents: 10000}, f.err
}
func (f *fakeOrderClient) ListOrderRecords(context.Context, *orderv1.ListOrderRecordsRequest, ...grpc.CallOption) (*orderv1.ListOrderRecordsResponse, error) {
	return &orderv1.ListOrderRecordsResponse{}, f.err
}

type fakeProductClient struct{ err error }

func (f *fakeProductClient) ListProducts(context.Context, *productv1.ListProductsRequest, ...grpc.CallOption) (*productv1.ListProductsResponse, error) {
	return &productv1.ListProductsResponse{}, f.err
}
func (f *fakeProductClient) CreateProduct(context.Context, *productv1.CreateProductRequest, ...grpc.CallOption) (*productv1.ProductResponse, error) {
	return &productv1.ProductResponse{Id: "P-1"}, f.err
}
func (f *fakeProductClient) UpdateProduct(context.Context, *productv1.UpdateProductRequest, ...grpc.CallOption) (*productv1.ProductResponse, error) {
	return &productv1.ProductResponse{Id: "P-1"}, f.err
}
func (f *fakeProductClient) GetProduct(context.Context, *productv1.GetProductRequest, ...grpc.CallOption) (*productv1.ProductResponse, error) {
	return &productv1.ProductResponse{Id: "P-1"}, f.err
}
func (f *fakeProductClient) ReserveStock(context.Context, *productv1.ReserveStockRequest, ...grpc.CallOption) (*productv1.ReserveStockResponse, error) {
	return &productv1.ReserveStockResponse{Ok: true}, f.err
}
func (f *fakeProductClient) ReleaseStock(context.Context, *productv1.ReleaseStockRequest, ...grpc.CallOption) (*productv1.ReleaseStockResponse, error) {
	return &productv1.ReleaseStockResponse{Ok: true}, f.err
}
func (f *fakeProductClient) ConfirmStock(context.Context, *productv1.ConfirmStockRequest, ...grpc.CallOption) (*productv1.ConfirmStockResponse, error) {
	return &productv1.ConfirmStockResponse{Ok: true}, f.err
}
func (f *fakeProductClient) RestoreStock(context.Context, *productv1.RestoreStockRequest, ...grpc.CallOption) (*productv1.RestoreStockResponse, error) {
	return &productv1.RestoreStockResponse{Ok: true}, f.err
}

type fakePaymentClient struct{ err error }

func (f *fakePaymentClient) Pay(context.Context, *paymentv1.PayRequest, ...grpc.CallOption) (*paymentv1.PayResponse, error) {
	return &paymentv1.PayResponse{Accepted: true}, f.err
}
func (f *fakePaymentClient) Refund(context.Context, *paymentv1.RefundRequest, ...grpc.CallOption) (*paymentv1.RefundResponse, error) {
	return &paymentv1.RefundResponse{Success: true}, f.err
}
func (f *fakePaymentClient) SetChannelConfig(context.Context, *paymentv1.SetChannelConfigRequest, ...grpc.CallOption) (*paymentv1.ChannelConfigResponse, error) {
	return &paymentv1.ChannelConfigResponse{PayBehavior: "no_notify"}, f.err
}
func (f *fakePaymentClient) GetChannelConfig(context.Context, *paymentv1.GetChannelConfigRequest, ...grpc.CallOption) (*paymentv1.ChannelConfigResponse, error) {
	return &paymentv1.ChannelConfigResponse{PayBehavior: "success"}, f.err
}
func (f *fakePaymentClient) ListChannelRecords(context.Context, *paymentv1.ListChannelRecordsRequest, ...grpc.CallOption) (*paymentv1.ListChannelRecordsResponse, error) {
	return &paymentv1.ListChannelRecordsResponse{}, f.err
}

type fakeWarehouseClient struct{ err error }

func (f *fakeWarehouseClient) ListPendingShip(context.Context, *warehousev1.ListPendingShipRequest, ...grpc.CallOption) (*warehousev1.ListPendingShipResponse, error) {
	return &warehousev1.ListPendingShipResponse{}, f.err
}
func (f *fakeWarehouseClient) Ship(context.Context, *warehousev1.ShipRequest, ...grpc.CallOption) (*warehousev1.ShipResponse, error) {
	return &warehousev1.ShipResponse{Success: true}, f.err
}

type fakeUserClient struct{ err error }

func (f *fakeUserClient) Register(context.Context, *userv1.RegisterRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "U-1", Role: "buyer"}, f.err
}
func (f *fakeUserClient) Login(context.Context, *userv1.LoginRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "U-1", Role: "buyer"}, f.err
}
func (f *fakeUserClient) GetUser(context.Context, *userv1.GetUserRequest, ...grpc.CallOption) (*userv1.UserResponse, error) {
	return &userv1.UserResponse{Id: "U-1"}, f.err
}

type fakeNoteClient struct{ err error }

func (f *fakeNoteClient) CreateNote(context.Context, *notev1.CreateNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "N-1"}, f.err
}
func (f *fakeNoteClient) GetNote(context.Context, *notev1.GetNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "N-1"}, f.err
}
func (f *fakeNoteClient) PublishNote(context.Context, *notev1.PublishNoteRequest, ...grpc.CallOption) (*notev1.NoteResponse, error) {
	return &notev1.NoteResponse{Id: "N-1"}, f.err
}
