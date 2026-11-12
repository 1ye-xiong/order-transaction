package service

import (
	"context"
	"errors"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"testing"
	"time"
)

type auditSelectiveStock struct {
	fakeStock
	attempted []string
}

func (s *auditSelectiveStock) Release(_ context.Context, no string) error {
	s.attempted = append(s.attempted, no)
	if no == "broken" {
		return errors.New("product unavailable for this order")
	}
	return nil
}
func TestAuditReconcileFailedOrderDoesNotStarveLaterOrders(t *testing.T) {
	r := newFakeRepo()
	for _, no := range []string{"broken", "healthy"} {
		if err := r.Create(ctx(), &entity.Order{OrderNo: no, Status: entity.StatusCancelled, PayExpireAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	stock := &auditSelectiveStock{}
	s := newTestService(r, &fakeChannel{}, stock)
	if err := s.reconcileStock(ctx()); err == nil {
		t.Fatal("failed order must remain visible")
	}
	if len(stock.attempted) != 2 || stock.attempted[1] != "healthy" {
		t.Fatalf("later order starved: %v", stock.attempted)
	}
	records, err := r.ListRecords(ctx(), "healthy")
	if err != nil || len(records) != 1 || records[0].Detail != "stock-completed:release" {
		t.Fatalf("healthy completion missing: %+v %v", records, err)
	}
	if err := s.reconcileStock(ctx()); err == nil {
		t.Fatal("broken order must retry")
	}
	if len(stock.attempted) != 3 || stock.attempted[2] != "broken" {
		t.Fatalf("successful order should not replay: %v", stock.attempted)
	}
}
