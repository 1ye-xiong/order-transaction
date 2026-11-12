package handler

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
	orderrepo "github.com/BwCloudWeGo/bw-cli/internal/order/repo"
	orderservice "github.com/BwCloudWeGo/bw-cli/internal/order/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
)

type stubChannel struct{}

func (stubChannel) Pay(_ context.Context, orderNo string, _ string, _ int64) (string, error) {
	return "CH-" + orderNo, nil
}

func (stubChannel) Refund(_ context.Context, _ string, _ string, _ int64, _ string) (bool, string, error) {
	return true, "success", nil
}

type stubStock struct{}

func (stubStock) Reserve(_ context.Context, _ string, _ string, _ int64) error { return nil }
func (stubStock) Release(_ context.Context, _ string) error                    { return nil }
func (stubStock) Confirm(_ context.Context, _ string) error                    { return nil }
func (stubStock) Restore(_ context.Context, _ string) error                    { return nil }
func (stubStock) GetProduct(_ context.Context, _ string) (string, int64, error) {
	return "商品", 10000, nil
}

func userCtx(userID string, role string) context.Context {
	md := metadata.Pairs(grpcx.MetadataUserID, userID, grpcx.MetadataUserRole, role)
	return metadata.NewIncomingContext(context.Background(), md)
}

func newTestServer(t *testing.T) *Server {
	svc, _ := newTestService(t)
	return NewServer(svc, nil)
}

func newTestService(t *testing.T) (*orderservice.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := orderrepo.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := orderservice.NewService(orderrepo.NewGormRepository(db), stubChannel{}, stubStock{}, 10*time.Minute, "test-secret", nil)
	return svc, db
}

func createOrderFor(t *testing.T, s *Server, buyerID string) *orderv1.OrderResponse {
	t.Helper()
	resp, err := s.CreateOrder(userCtx(buyerID, "buyer"), &orderv1.CreateOrderRequest{ProductId: "P1", Quantity: 1})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	return resp
}

func payToPaid(t *testing.T, s *Server, buyerID string, orderNo string) {
	t.Helper()
	if _, err := s.PayOrder(userCtx(buyerID, "buyer"), &orderv1.PayOrderRequest{OrderNo: orderNo}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	sig := orderservice.SignPayload("test-secret", orderNo, 10000, "CH-"+orderNo, "SUCCESS", time.Now().Unix())
	resp, err := s.HandlePaymentNotify(context.Background(), &orderv1.HandlePaymentNotifyRequest{
		OrderNo: orderNo, AmountCents: 10000, ChannelTradeNo: "CH-" + orderNo, Status: "SUCCESS",
		NotifiedAtUnix: time.Now().Unix(), Signature: sig,
	})
	if err != nil || !resp.Accepted {
		t.Fatalf("notify: %v / %+v", err, resp)
	}
}

func isForbidden(err error) bool {
	return apperrors.GRPCCode(err) == codes.PermissionDenied
}

func TestBuyerFlow(t *testing.T) {
	s := newTestServer(t)
	order := createOrderFor(t, s, "buyer1")
	if order.Status != "PENDING_PAYMENT" || order.AmountCents != 10000 {
		t.Fatalf("order = %+v", order)
	}
	if _, err := s.GetOrder(userCtx("buyer2", "buyer"), &orderv1.GetOrderRequest{OrderNo: order.OrderNo}); !isForbidden(err) {
		t.Fatalf("owner check failed: %v", err)
	}
	list, err := s.ListMyOrders(userCtx("buyer1", "buyer"), &orderv1.ListMyOrdersRequest{Page: 1, PageSize: 10})
	if err != nil || list.Total != 1 {
		t.Fatalf("list: %v / %d", err, list.Total)
	}
	if _, err := s.CreateOrder(userCtx("cs01", "cs"), &orderv1.CreateOrderRequest{ProductId: "P1", Quantity: 1}); err == nil {
		t.Fatal("non-buyer should be denied")
	}
	payToPaid(t, s, "buyer1", order.OrderNo)
	got, _ := s.GetOrder(userCtx("buyer1", "buyer"), &orderv1.GetOrderRequest{OrderNo: order.OrderNo})
	if got.Status != "PAID" {
		t.Fatalf("status = %s", got.Status)
	}
	canceled, err := s.CancelOrder(userCtx("buyer1", "buyer"), &orderv1.CancelOrderRequest{OrderNo: order.OrderNo})
	if err != nil || canceled.Status != "PAID" {
		t.Fatalf("cancel paid order should keep status: %v / %s", err, canceled.Status)
	}
}

func TestStaffFlow(t *testing.T) {
	s := newTestServer(t)
	order := createOrderFor(t, s, "buyer1")
	payToPaid(t, s, "buyer1", order.OrderNo)

	if _, err := s.ListShippableOrders(userCtx("buyer1", "buyer"), &orderv1.ListShippableOrdersRequest{}); err == nil {
		t.Fatal("buyer should not list shippable orders")
	}
	shippable, err := s.ListShippableOrders(userCtx("wh01", "warehouse"), &orderv1.ListShippableOrdersRequest{})
	if err != nil || len(shippable.Items) != 1 {
		t.Fatalf("shippable: %v / %d", err, len(shippable.Items))
	}
	if _, err := s.ShipOrder(userCtx("wh01", "warehouse"), &orderv1.ShipOrderRequest{OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF001"}); err != nil {
		t.Fatalf("ship: %v", err)
	}
	if _, err := s.ShipOrder(userCtx("wh01", "warehouse"), &orderv1.ShipOrderRequest{OrderNo: order.OrderNo, Carrier: "SF", TrackingNo: "SF001"}); err == nil {
		t.Fatal("duplicate ship should fail")
	}
	order2 := createOrderFor(t, s, "buyer2")
	payToPaid(t, s, "buyer2", order2.OrderNo)
	if _, err := s.ShipOrder(userCtx("wh01", "warehouse"), &orderv1.ShipOrderRequest{OrderNo: order2.OrderNo, Carrier: "", TrackingNo: ""}); err == nil {
		t.Fatal("missing tracking no should fail")
	}
}

func TestCSAndOpsFlow(t *testing.T) {
	s := newTestServer(t)
	order := createOrderFor(t, s, "buyer1")
	payToPaid(t, s, "buyer1", order.OrderNo)
	if _, err := s.ApplyRefund(userCtx("buyer1", "buyer"), &orderv1.ApplyRefundRequest{OrderNo: order.OrderNo, Reason: "x"}); err != nil {
		t.Fatalf("apply refund: %v", err)
	}
	if _, err := s.ListRefundRequests(userCtx("buyer1", "buyer"), &orderv1.ListRefundRequestsRequest{}); err == nil {
		t.Fatal("buyer should not list refunds")
	}
	reqs, err := s.ListRefundRequests(userCtx("cs01", "cs"), &orderv1.ListRefundRequestsRequest{Status: "PENDING"})
	if err != nil || len(reqs.Items) != 1 {
		t.Fatalf("refund requests: %v / %d", err, len(reqs.Items))
	}
	reviewed, err := s.ReviewRefund(userCtx("cs01", "cs"), &orderv1.ReviewRefundRequest{RefundId: reqs.Items[0].Id, Approve: true, Reason: "ok"})
	if err != nil || reviewed.Status != "REFUNDED" {
		t.Fatalf("review: %v / %s", err, reviewed.Status)
	}
	order2 := createOrderFor(t, s, "buyer2")
	payToPaid(t, s, "buyer2", order2.OrderNo)
	_, _ = s.ApplyRefund(userCtx("buyer2", "buyer"), &orderv1.ApplyRefundRequest{OrderNo: order2.OrderNo, Reason: "y"})
	reqs2, _ := s.ListRefundRequests(userCtx("cs01", "cs"), &orderv1.ListRefundRequestsRequest{Status: "PENDING"})
	if _, err := s.ReviewRefund(userCtx("cs01", "cs"), &orderv1.ReviewRefundRequest{RefundId: reqs2.Items[0].Id, Approve: false, Reason: ""}); err == nil {
		t.Fatal("reject without reason should fail")
	}
	if _, err := s.OpsSummary(userCtx("buyer1", "buyer"), &orderv1.OpsSummaryRequest{}); err == nil {
		t.Fatal("buyer should not read ops summary")
	}
	summary, err := s.OpsSummary(userCtx("ops01", "ops"), &orderv1.OpsSummaryRequest{})
	if err != nil || summary.TotalPaidCents != 20000 || summary.Refunded != 1 {
		t.Fatalf("summary: %v / %+v", err, summary)
	}
	records, err := s.ListOrderRecords(userCtx("ops01", "ops"), &orderv1.ListOrderRecordsRequest{OrderNo: order.OrderNo})
	if err != nil || len(records.Items) < 3 {
		t.Fatalf("records: %v / %d", err, len(records.Items))
	}
	if _, err := s.ListPayExceptions(userCtx("cs01", "cs"), &orderv1.ListPayExceptionsRequest{}); err != nil {
		t.Fatalf("exceptions: %v", err)
	}
	if _, err := s.RetryRefund(userCtx("cs01", "cs"), &orderv1.RetryRefundRequest{OrderNo: order.OrderNo}); err == nil {
		t.Fatal("retry on non-REFUND_FAILED should fail")
	}
}

// 迟到支付：超时关闭后收到成功通知 → 异常登记 → 客服退款并关闭异常。
func TestLatePaymentExceptionFlow(t *testing.T) {
	svc, db := newTestService(t)
	s := NewServer(svc, nil)
	order := createOrderFor(t, s, "buyer1")

	if err := db.Model(&dbmodel.OrderModel{}).Where("order_no = ?", order.OrderNo).
		Update("pay_expire_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire order: %v", err)
	}

	tradeNo := "CH-LATE-" + order.OrderNo
	now := time.Now().Unix()
	sig := orderservice.SignPayload("test-secret", order.OrderNo, 10000, tradeNo, "SUCCESS", now)
	resp, err := s.HandlePaymentNotify(context.Background(), &orderv1.HandlePaymentNotifyRequest{
		OrderNo: order.OrderNo, AmountCents: 10000, ChannelTradeNo: tradeNo, Status: "SUCCESS",
		NotifiedAtUnix: now, Signature: sig,
	})
	if err != nil || !resp.Accepted {
		t.Fatalf("late notify: %v / %+v", err, resp)
	}

	after, err := s.GetOrder(userCtx("buyer1", "buyer"), &orderv1.GetOrderRequest{OrderNo: order.OrderNo})
	if err != nil || after.Status != "CLOSED_TIMEOUT" {
		t.Fatalf("order should stay closed: %v / %s", err, after.Status)
	}

	exs, err := s.ListPayExceptions(userCtx("cs01", "cs"), &orderv1.ListPayExceptionsRequest{Status: "PENDING"})
	if err != nil || len(exs.Items) != 1 {
		t.Fatalf("exceptions: %v / %d", err, len(exs.Items))
	}
	ex := exs.Items[0]
	if ex.Reason != "late_payment" || ex.ChannelTradeNo != tradeNo || ex.HandledAt != "" {
		t.Fatalf("exception = %+v", ex)
	}

	refunded, err := s.RefundPayException(userCtx("cs01", "cs"), &orderv1.RefundPayExceptionRequest{ExceptionId: ex.Id})
	if err != nil || !refunded.Success {
		t.Fatalf("refund exception: %v / %+v", err, refunded)
	}
	if refunded.Exception.Status != "REFUNDED" || refunded.Exception.HandledAt == "" {
		t.Fatalf("refunded exception = %+v", refunded.Exception)
	}

	again, err := s.RefundPayException(userCtx("cs01", "cs"), &orderv1.RefundPayExceptionRequest{ExceptionId: ex.Id})
	if err != nil || again.Success || again.Message == "" {
		t.Fatalf("重复退款应返回已处理: %v / %+v", err, again)
	}
}
