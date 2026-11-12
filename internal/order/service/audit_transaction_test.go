package service

import (
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	orderrepo "github.com/BwCloudWeGo/bw-cli/internal/order/repo"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestAuditBusinessStateAndRecordRollbackTogether(t *testing.T) {
	for _, action := range []string{"cancel", "timeout", "ship", "pay", "refund-success", "refund-failure", "apply", "review"} {
		t.Run(action, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			conn, _ := db.DB()
			conn.SetMaxOpenConns(1)
			defer conn.Close()
			if err := orderrepo.AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			r := orderrepo.NewGormRepository(db)
			ch := &fakeChannel{}
			stock := &fakeStock{}
			s := newTestService(nil, ch, stock)
			s.repo = r
			state := entity.StatusPendingPayment
			if action == "ship" || action == "apply" {
				state = entity.StatusPaid
			}
			if action == "refund-success" || action == "refund-failure" || action == "review" {
				state = entity.StatusRefundRequested
			}
			expiry := time.Now().UTC().Add(time.Minute)
			if action == "timeout" {
				expiry = time.Now().UTC().Add(-time.Minute)
			}
			o := &entity.Order{OrderNo: "audit-tx", BuyerID: "buyer1", Status: state, UnitPriceCents: 10000, Quantity: 1, PayExpireAt: expiry, ChannelTradeNo: "paid-before"}
			if action == "pay" {
				o.ChannelTradeNo = ""
			}
			if err := r.Create(ctx(), o); err != nil {
				t.Fatal(err)
			}
			req := &entity.RefundRequest{OrderNo: o.OrderNo, BuyerID: o.BuyerID, Status: entity.RefundPending, Reason: "request"}
			if action == "review" {
				if err := r.CreateRefundRequest(ctx(), req); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Exec("CREATE TRIGGER fail_audit BEFORE INSERT ON order_records BEGIN SELECT RAISE(ABORT,'audit disk failure'); END").Error; err != nil {
				t.Fatal(err)
			}
			var runErr error
			switch action {
			case "cancel":
				_, runErr = s.CancelOrder(ctx(), o.BuyerID, o.OrderNo, "reason")
			case "timeout":
				_, runErr = s.CloseExpiredOrders(ctx())
			case "ship":
				_, runErr = s.ShipOrder(ctx(), dto.ShipCommand{OrderNo: o.OrderNo, Carrier: "carrier", TrackingNo: "tracking", Operator: "warehouse"})
			case "pay":
				req := NotifyInput{OrderNo: o.OrderNo, AmountCents: 10000, ChannelTradeNo: "new-paid", Status: "SUCCESS", NotifiedAtUnix: time.Now().Unix()}
				req.Signature = SignPayload("test-secret", req.OrderNo, req.AmountCents, req.ChannelTradeNo, req.Status, req.NotifiedAtUnix)
				ok, _ := s.HandlePaymentNotify(ctx(), req)
				if ok {
					t.Fatal("payment must report audit failure")
				}
			case "refund-success", "refund-failure":
				if action == "refund-failure" {
					ch.succeedFrom = 2
				}
				_, runErr = s.executeRefund(ctx(), o.OrderNo, "RF-audit", "cs", "verified")
			case "apply":
				_, runErr = s.ApplyRefund(ctx(), dto.ApplyRefundCommand{OrderNo: o.OrderNo, BuyerID: o.BuyerID, Reason: "reason"})
			case "review":
				_, runErr = s.ReviewRefund(ctx(), dto.ReviewRefundCommand{RefundID: req.ID, ReviewerID: "cs", Reason: "verified", Approve: false})
			}
			if action != "pay" && runErr == nil {
				t.Fatal("must return audit error")
			}
			got, err := r.FindByOrderNo(ctx(), o.OrderNo)
			if err != nil || got.Status != state {
				t.Fatalf("state committed without audit: %+v err=%v", got, err)
			}
			if err := db.Exec("DROP TRIGGER fail_audit").Error; err != nil {
				t.Fatal(err)
			}
			if action == "ship" {
				if _, err := s.ShipOrder(ctx(), dto.ShipCommand{OrderNo: o.OrderNo, Carrier: "carrier", TrackingNo: "tracking", Operator: "warehouse"}); err != nil {
					t.Fatal(err)
				}
				records, _ := r.ListRecords(ctx(), o.OrderNo)
				if len(records) != 1 {
					t.Fatalf("retry must persist exactly one audit, got %d", len(records))
				}
			}
		})
	}
}
