package handler

import (
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func buyerToken(t *testing.T) string {
	return gwToken(t, middleware.JWTClaims{UserID: "U-1", Role: "buyer"})
}

func TestOrderGatewayRoutes(t *testing.T) {
	order := &fakeOrderClient{}
	r := newTestGateway(t, order, &fakeProductClient{}, &fakePaymentClient{}, &fakeWarehouseClient{}, &fakeUserClient{}, &fakeNoteClient{})
	token := buyerToken(t)

	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders", token, `{"product_id":"P1","quantity":1}`); w.Code != http.StatusCreated {
		t.Fatalf("create order = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/orders/mine", token, ""); w.Code != http.StatusOK {
		t.Fatalf("list mine = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/orders/ORD-1", token, ""); w.Code != http.StatusOK {
		t.Fatalf("get order = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders/ORD-1/cancel", token, ""); w.Code != http.StatusOK {
		t.Fatalf("cancel = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders/ORD-1/pay", token, ""); w.Code != http.StatusOK {
		t.Fatalf("pay = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders/ORD-1/refund", token, `{"reason":"不想要了"}`); w.Code != http.StatusOK {
		t.Fatalf("refund = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders", token, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/orders/ORD-1/refund", token, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("refund without reason = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/orders/ORD-1", gwToken(t, middleware.JWTClaims{}), ""); w.Code != http.StatusOK {
		t.Fatalf("no-claims token should still pass jwt: %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/orders/ORD-1", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token = %d", w.Code)
	}
}

func TestOrderGatewayDownstreamErrors(t *testing.T) {
	err := status.Error(codes.NotFound, "order_not_found|order not found")
	r := newTestGateway(t, &fakeOrderClient{err: err}, &fakeProductClient{}, &fakePaymentClient{}, &fakeWarehouseClient{}, &fakeUserClient{}, &fakeNoteClient{})
	token := buyerToken(t)

	w := gwDo(t, r, http.MethodGet, "/api/v1/orders/ORD-1", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("downstream not found = %d / %s", w.Code, w.Body.String())
	}
	payload := gwDecode(t, w)
	if payload["error"] == nil || !strings.Contains(w.Body.String(), "order_not_found") {
		t.Fatalf("error body = %s", w.Body.String())
	}
}

func TestProductGatewayRoutes(t *testing.T) {
	product := &fakeProductClient{}
	r := newTestGateway(t, &fakeOrderClient{}, product, &fakePaymentClient{}, &fakeWarehouseClient{}, &fakeUserClient{}, &fakeNoteClient{})
	token := buyerToken(t)

	if w := gwDo(t, r, http.MethodGet, "/api/v1/products?include_off_sale=true", token, ""); w.Code != http.StatusOK {
		t.Fatalf("list = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/products", token, `{"name":"商品","price_cents":10000,"stock_total":5}`); w.Code != http.StatusCreated {
		t.Fatalf("create = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/products", token, `{"price_cents":10000}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPut, "/api/v1/products/P-1", token, `{"set_price":true,"price_cents":8000}`); w.Code != http.StatusOK {
		t.Fatalf("update = %d / %s", w.Code, w.Body.String())
	}
}

func TestChannelGatewayRoutes(t *testing.T) {
	payment := &fakePaymentClient{}
	r := newTestGateway(t, &fakeOrderClient{}, &fakeProductClient{}, payment, &fakeWarehouseClient{}, &fakeUserClient{}, &fakeNoteClient{})

	if w := gwDo(t, r, http.MethodGet, "/api/v1/channel/config", "", ""); w.Code != http.StatusOK {
		t.Fatalf("get config = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPut, "/api/v1/channel/config", "", `{"pay_behavior":"no_notify","notify_count":2,"refund_success_from_attempt":1}`); w.Code != http.StatusOK {
		t.Fatalf("set config = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPut, "/api/v1/channel/config", "", `{"pay_behavior":"boom"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid config = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/channel/records", "", ""); w.Code != http.StatusOK {
		t.Fatalf("records = %d / %s", w.Code, w.Body.String())
	}
}

func TestStaffGatewayRoutes(t *testing.T) {
	order := &fakeOrderClient{}
	warehouse := &fakeWarehouseClient{}
	r := newTestGateway(t, order, &fakeProductClient{}, &fakePaymentClient{}, warehouse, &fakeUserClient{}, &fakeNoteClient{})
	token := gwToken(t, middleware.JWTClaims{UserID: "WH-1", Role: "warehouse"})

	if w := gwDo(t, r, http.MethodGet, "/api/v1/warehouse/orders", token, ""); w.Code != http.StatusOK {
		t.Fatalf("pending ship = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/warehouse/orders/ORD-1/ship", token, `{"carrier":"SF","tracking_no":"SF1"}`); w.Code != http.StatusOK {
		t.Fatalf("ship = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/warehouse/orders/ORD-1/ship", token, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("ship missing params = %d / %s", w.Code, w.Body.String())
	}

	csToken := gwToken(t, middleware.JWTClaims{UserID: "CS-1", Role: "cs"})
	if w := gwDo(t, r, http.MethodGet, "/api/v1/cs/refunds?status=PENDING", csToken, ""); w.Code != http.StatusOK {
		t.Fatalf("refund requests = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/cs/refunds/1/review", csToken, `{"approve":true,"reason":"ok"}`); w.Code != http.StatusOK {
		t.Fatalf("review = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/cs/refunds/1/review", csToken, `{not-json`); w.Code != http.StatusBadRequest {
		t.Fatalf("review bad body = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/cs/orders/ORD-1/refund-retry", csToken, ""); w.Code != http.StatusOK {
		t.Fatalf("retry = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/cs/pay-exceptions", csToken, ""); w.Code != http.StatusOK {
		t.Fatalf("pay exceptions = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/cs/pay-exceptions/1/refund", csToken, ""); w.Code != http.StatusOK {
		t.Fatalf("refund exception = %d / %s", w.Code, w.Body.String())
	}

	opsToken := gwToken(t, middleware.JWTClaims{UserID: "OPS-1", Role: "ops"})
	if w := gwDo(t, r, http.MethodGet, "/api/v1/ops/summary", opsToken, ""); w.Code != http.StatusOK {
		t.Fatalf("summary = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/ops/orders/ORD-1/records", opsToken, ""); w.Code != http.StatusOK {
		t.Fatalf("records = %d / %s", w.Code, w.Body.String())
	}
}

func TestUserGatewayRoutes(t *testing.T) {
	user := &fakeUserClient{}
	r := newTestGateway(t, &fakeOrderClient{}, &fakeProductClient{}, &fakePaymentClient{}, &fakeWarehouseClient{}, user, &fakeNoteClient{})

	if w := gwDo(t, r, http.MethodPost, "/api/v1/users/register", "", `{"account":"a","display_name":"A","password":"123456"}`); w.Code != http.StatusCreated {
		t.Fatalf("register = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/users/register", "", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid register = %d / %s", w.Code, w.Body.String())
	}
	w := gwDo(t, r, http.MethodPost, "/api/v1/users/login", "", `{"account":"a","password":"123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login = %d / %s", w.Code, w.Body.String())
	}
	payload := gwDecode(t, w)
	data, _ := payload["data"].(map[string]interface{})
	if data == nil || data["token"] == "" {
		t.Fatalf("login payload = %s", w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/users/login", "", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid login = %d / %s", w.Code, w.Body.String())
	}

	emptyClaims := gwToken(t, middleware.JWTClaims{})
	if w := gwDo(t, r, http.MethodGet, "/api/v1/users/me", emptyClaims, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("me without claims = %d / %s", w.Code, w.Body.String())
	}
	token := buyerToken(t)
	if w := gwDo(t, r, http.MethodGet, "/api/v1/users/me", token, ""); w.Code != http.StatusOK {
		t.Fatalf("me = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/users/U-1", token, ""); w.Code != http.StatusOK {
		t.Fatalf("get user = %d / %s", w.Code, w.Body.String())
	}
}

func TestNoteGatewayRoutes(t *testing.T) {
	note := &fakeNoteClient{}
	r := newTestGateway(t, &fakeOrderClient{}, &fakeProductClient{}, &fakePaymentClient{}, &fakeWarehouseClient{}, &fakeUserClient{}, note)

	if w := gwDo(t, r, http.MethodPost, "/api/v1/notes", "", `{"author_id":"U-1","title":"t","content":"c"}`); w.Code != http.StatusCreated {
		t.Fatalf("create note = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodGet, "/api/v1/notes/N-1", "", ""); w.Code != http.StatusOK {
		t.Fatalf("get note = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/notes/publish", "", `{"author_id":"U-1","title":"t","content":"c"}`); w.Code != http.StatusOK {
		t.Fatalf("publish = %d / %s", w.Code, w.Body.String())
	}
	if w := gwDo(t, r, http.MethodPost, "/api/v1/notes", "", `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create = %d / %s", w.Code, w.Body.String())
	}
}

// 下游 gRPC 出错时所有接口都应把错误透传成统一错误外壳。
func TestGatewayDownstreamErrorPropagation(t *testing.T) {
	err := status.Error(codes.NotFound, "not_found|missing")
	r := newTestGateway(t,
		&fakeOrderClient{err: err},
		&fakeProductClient{err: err},
		&fakePaymentClient{err: err},
		&fakeWarehouseClient{err: err},
		&fakeUserClient{err: err},
		&fakeNoteClient{err: err},
	)
	token := buyerToken(t)
	csToken := gwToken(t, middleware.JWTClaims{UserID: "CS-1", Role: "cs"})

	cases := []struct{ method, path, token, body string }{
		{http.MethodGet, "/api/v1/products", token, ""},
		{http.MethodPost, "/api/v1/products", token, `{"name":"a","price_cents":1,"stock_total":1}`},
		{http.MethodPut, "/api/v1/products/P-1", token, `{"set_price":true,"price_cents":2}`},
		{http.MethodPost, "/api/v1/orders", token, `{"product_id":"P1","quantity":1}`},
		{http.MethodGet, "/api/v1/orders/mine", token, ""},
		{http.MethodPost, "/api/v1/orders/ORD-1/cancel", token, ""},
		{http.MethodPost, "/api/v1/orders/ORD-1/pay", token, ""},
		{http.MethodPost, "/api/v1/orders/ORD-1/refund", token, `{"reason":"x"}`},
		{http.MethodGet, "/api/v1/warehouse/orders", csToken, ""},
		{http.MethodPost, "/api/v1/warehouse/orders/ORD-1/ship", csToken, `{"carrier":"SF","tracking_no":"SF1"}`},
		{http.MethodGet, "/api/v1/cs/refunds", csToken, ""},
		{http.MethodPost, "/api/v1/cs/refunds/1/review", csToken, `{"approve":true,"reason":"ok"}`},
		{http.MethodPost, "/api/v1/cs/orders/ORD-1/refund-retry", csToken, ""},
		{http.MethodGet, "/api/v1/cs/pay-exceptions", csToken, ""},
		{http.MethodPost, "/api/v1/cs/pay-exceptions/1/refund", csToken, ""},
		{http.MethodGet, "/api/v1/ops/summary", csToken, ""},
		{http.MethodGet, "/api/v1/ops/orders/ORD-1/records", csToken, ""},
		{http.MethodGet, "/api/v1/channel/config", "", ""},
		{http.MethodPut, "/api/v1/channel/config", "", `{"pay_behavior":"success","notify_count":1,"refund_success_from_attempt":1}`},
		{http.MethodGet, "/api/v1/channel/records", "", ""},
		{http.MethodPost, "/api/v1/users/register", "", `{"account":"a","display_name":"A","password":"123456"}`},
		{http.MethodPost, "/api/v1/users/login", "", `{"account":"a","password":"123456"}`},
		{http.MethodGet, "/api/v1/users/me", token, ""},
		{http.MethodGet, "/api/v1/users/U-1", token, ""},
		{http.MethodPost, "/api/v1/notes", "", `{"author_id":"U-1","title":"t","content":"c"}`},
		{http.MethodGet, "/api/v1/notes/N-1", "", ""},
		{http.MethodPost, "/api/v1/notes/publish", "", `{"author_id":"U-1","title":"t","content":"c"}`},
	}
	for _, tc := range cases {
		w := gwDo(t, r, tc.method, tc.path, tc.token, tc.body)
		if w.Code < 400 {
			t.Fatalf("%s %s 应透传下游错误, got %d / %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "not_found") {
			t.Fatalf("%s %s 错误外壳应包含原始错误码: %s", tc.method, tc.path, w.Body.String())
		}
	}
}

func TestNewHandlersAggregation(t *testing.T) {
	h := NewHandlers(&fakeProductClient{}, &fakeOrderClient{}, &fakePaymentClient{}, NewStaffHandler(&fakeOrderClient{}, &fakeWarehouseClient{}, zap.NewNop()), zap.NewNop())
	if h == nil || h.Product == nil || h.Order == nil || h.Staff == nil || h.Channel == nil {
		t.Fatal("handlers 聚合不完整")
	}
}
