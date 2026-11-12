package repo

import (
	"context"
	"sync"
	"testing"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
)

func auditReceipt(t *testing.T, r *GormRepository, trade string, amount int64, result string) {
	t.Helper()
	if err := r.SaveRecord(context.Background(), &entity.ChannelRecord{Type: entity.RecordTypePayment, ChannelTradeNo: trade, OrderNo: "O-" + trade, AmountCents: amount, Result: result}); err != nil {
		t.Fatal(err)
	}
}

func TestAuditRefundRequiresConfirmedMatchingPayment(t *testing.T) {
	r := NewGormRepository(newTestDB(t))
	auditReceipt(t, r, "CH-OK", 10000, "success")
	auditReceipt(t, r, "CH-FAILED", 10000, "failed")
	for _, c := range []struct {
		name, refund, trade string
		amount              int64
	}{
		{"empty refund", "", "CH-OK", 10000},
		{"empty trade", "R", "", 10000},
		{"zero", "R", "CH-OK", 0},
		{"negative", "R", "CH-OK", -1},
		{"overpayment", "R", "CH-OK", 10001},
		{"partial", "R", "CH-OK", 9999},
		{"unknown payment", "R", "CH-UNKNOWN", 10000},
		{"failed payment", "R", "CH-FAILED", 10000},
	} {
		t.Run(c.name, func(t *testing.T) {
			ok, _, err := r.ExecuteRefund(context.Background(), c.refund, c.trade, c.amount, "核实退款")
			if ok || err == nil {
				t.Fatalf("invalid refund accepted: ok=%v err=%v", ok, err)
			}
		})
	}
	records, err := r.ListRecords(context.Background())
	if err != nil || len(records) != 2 {
		t.Fatalf("invalid refunds must not create money records: %v / %+v", err, records)
	}
}

func TestAuditRefundConcurrentRetriesCreateOneSuccessfulRefund(t *testing.T) {
	r := NewGormRepository(newTestDB(t))
	auditReceipt(t, r, "CH-CONCURRENT", 10000, "success")
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := r.ExecuteRefund(context.Background(), "RF-ONE", "CH-CONCURRENT", 10000, "核实退款")
			if err != nil {
				errs <- err
			} else if !ok {
				t.Error("valid duplicate refund should return its original success")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	records, err := r.ListRecords(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var refunds int
	var refunded int64
	for _, record := range records {
		if record.Type == entity.RecordTypeRefund && record.Result == "success" {
			refunds++
			refunded += record.AmountCents
		}
	}
	if refunds != 1 || refunded != 10000 {
		t.Fatalf("duplicate refund: count=%d amount=%d", refunds, refunded)
	}
	if ok, _, err := r.ExecuteRefund(context.Background(), "RF-OTHER", "CH-CONCURRENT", 10000, "重复业务号"); ok || err == nil {
		t.Fatalf("new business ID must not refund same payment again: ok=%v err=%v", ok, err)
	}
}

func TestAuditFailedRefundKeepsBusinessIdentityAndRetries(t *testing.T) {
	r := NewGormRepository(newTestDB(t))
	auditReceipt(t, r, "CH-A", 10000, "success")
	auditReceipt(t, r, "CH-B", 10000, "success")
	if err := r.SaveConfig(context.Background(), entity.ChannelConfig{RefundSuccessFromAttempt: 2}); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := r.ExecuteRefund(context.Background(), "RF-RETRY", "CH-A", 10000, "核实"); ok || err != nil {
		t.Fatalf("first attempt must fail explicitly: %v / %v", ok, err)
	}
	if ok, _, err := r.ExecuteRefund(context.Background(), "RF-RETRY", "CH-B", 10000, "换交易号"); ok || err == nil {
		t.Fatalf("same refund ID must not switch payment: %v / %v", ok, err)
	}
	if ok, _, err := r.ExecuteRefund(context.Background(), "RF-RETRY", "CH-A", 10000, "核实重试"); !ok || err != nil {
		t.Fatalf("second attempt must succeed: %v / %v", ok, err)
	}
	if ok, _, err := r.ExecuteRefund(context.Background(), "RF-RETRY", "CH-A", 10000, "第三次"); !ok || err != nil {
		t.Fatalf("success must be stable: %v / %v", ok, err)
	}
	n, err := r.CountRefundAttempts(context.Background(), "RF-RETRY")
	if err != nil || n != 2 {
		t.Fatalf("successful retry must not create third attempt: %d / %v", n, err)
	}
}
