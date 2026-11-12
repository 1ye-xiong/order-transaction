package handler

import (
	"context"
	"testing"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"google.golang.org/grpc/codes"
)

func TestAuditBuyerCannotInvokeStaffOperations(t *testing.T) {
	s := newTestServer(t)
	buyer := userCtx("buyer1", "buyer")
	checks := map[string]func() error{
		"ship":             func() error { _, err := s.ShipOrder(buyer, &orderv1.ShipOrderRequest{}); return err },
		"review":           func() error { _, err := s.ReviewRefund(buyer, &orderv1.ReviewRefundRequest{}); return err },
		"retry":            func() error { _, err := s.RetryRefund(buyer, &orderv1.RetryRefundRequest{}); return err },
		"exceptions":       func() error { _, err := s.ListPayExceptions(buyer, &orderv1.ListPayExceptionsRequest{}); return err },
		"exception refund": func() error { _, err := s.RefundPayException(buyer, &orderv1.RefundPayExceptionRequest{}); return err },
		"records":          func() error { _, err := s.ListOrderRecords(buyer, &orderv1.ListOrderRecordsRequest{}); return err },
	}
	for name, call := range checks {
		t.Run(name, func(t *testing.T) {
			if err := call(); !isForbidden(err) {
				t.Fatalf("expected permission denied: %v", err)
			}
		})
	}
}

func TestAuditAnonymousCannotInvokeBuyerOperations(t *testing.T) {
	s := newTestServer(t)
	anon := context.Background()
	checks := map[string]func() error{
		"get":    func() error { _, err := s.GetOrder(anon, &orderv1.GetOrderRequest{}); return err },
		"list":   func() error { _, err := s.ListMyOrders(anon, &orderv1.ListMyOrdersRequest{}); return err },
		"cancel": func() error { _, err := s.CancelOrder(anon, &orderv1.CancelOrderRequest{}); return err },
		"pay":    func() error { _, err := s.PayOrder(anon, &orderv1.PayOrderRequest{}); return err },
		"refund": func() error { _, err := s.ApplyRefund(anon, &orderv1.ApplyRefundRequest{}); return err },
	}
	for name, call := range checks {
		t.Run(name, func(t *testing.T) {
			if err := call(); !isForbidden(err) {
				t.Fatalf("expected permission denied: %v", err)
			}
		})
	}
}

func TestAuditUnknownOrderAndInvalidQuantityReturnUsefulErrors(t *testing.T) {
	s := newTestServer(t)
	buyer := userCtx("buyer1", "buyer")
	if _, err := s.CreateOrder(buyer, &orderv1.CreateOrderRequest{ProductId: "P1", Quantity: 3}); apperrors.GRPCCode(err) != codes.InvalidArgument {
		t.Fatalf("invalid quantity must be invalid argument: %v", err)
	}
	checks := map[string]func() error{
		"cancel": func() error {
			_, err := s.CancelOrder(buyer, &orderv1.CancelOrderRequest{OrderNo: "missing"})
			return err
		},
		"pay": func() error { _, err := s.PayOrder(buyer, &orderv1.PayOrderRequest{OrderNo: "missing"}); return err },
		"refund": func() error {
			_, err := s.ApplyRefund(buyer, &orderv1.ApplyRefundRequest{OrderNo: "missing", Reason: "不需要"})
			return err
		},
	}
	for name, call := range checks {
		t.Run(name, func(t *testing.T) {
			if err := call(); apperrors.GRPCCode(err) != codes.NotFound {
				t.Fatalf("missing order must be not found: %v", err)
			}
		})
	}
}
