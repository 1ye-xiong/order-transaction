package repo

import (
	"context"
	"testing"
	"time"

	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newOrder(orderNo string) *entity.Order {
	now := time.Now().UTC()
	return &entity.Order{
		OrderNo: orderNo, BuyerID: "buyer1", ProductID: "P1", ProductName: "商品",
		UnitPriceCents: 10000, Quantity: 1, Status: entity.StatusPendingPayment,
		PayExpireAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now,
	}
}

func TestCreateAndFind(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	if err := repo.Create(ctx, newOrder("ORD-1")); err != nil {
		t.Fatalf("create: %v", err)
	}
	o, err := repo.FindByOrderNo(ctx, "ORD-1")
	if err != nil || o.BuyerID != "buyer1" || o.AmountCents() != 10000 {
		t.Fatalf("find: %v / %+v", err, o)
	}
	if _, err := repo.FindByOrderNo(ctx, "ORD-NONE"); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单应 ErrOrderNotFound, got %v", err)
	}
}

// 同一渠道交易号只能归属一个订单（唯一索引兜底）。
func TestMarkPaidTradeNoUnique(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-1"))
	_ = repo.Create(ctx, newOrder("ORD-2"))

	ok, err := repo.MarkPaid(ctx, "ORD-1", "CH-1", time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("MarkPaid: ok = %v, err = %v", ok, err)
	}
	_, err = repo.MarkPaid(ctx, "ORD-2", "CH-1", time.Now().UTC())
	if err == nil {
		t.Fatal("交易号已归属其他订单时应报唯一冲突")
	}
	o, _ := repo.FindByOrderNo(ctx, "ORD-2")
	if o.Status == entity.StatusPaid {
		t.Fatal("冲突时不应标记已付款")
	}
}

// MarkPaid 拒绝过期订单。
func TestMarkPaidExpired(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-EXP"))
	repo.db.Model(&dbmodel.OrderModel{}).Where("order_no = ?", "ORD-EXP").Update("pay_expire_at", time.Now().UTC().Add(-time.Minute))
	ok, err := repo.MarkPaid(ctx, "ORD-EXP", "CH-2", time.Now().UTC())
	if err != nil || ok {
		t.Fatalf("过期订单支付应失败: ok = %v, err = %v", ok, err)
	}
}

// CancelPending / CloseIfExpired 幂等。
func TestCancelAndCloseIdempotent(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-C"))

	if ok, _ := repo.CancelPending(ctx, "ORD-C", "买家取消", time.Now().UTC()); !ok {
		t.Fatal("首次取消应成功")
	}
	if ok, _ := repo.CancelPending(ctx, "ORD-C", "买家取消", time.Now().UTC()); ok {
		t.Fatal("重复取消应返回 false")
	}

	_ = repo.Create(ctx, newOrder("ORD-T"))
	repo.db.Model(&dbmodel.OrderModel{}).Where("order_no = ?", "ORD-T").Update("pay_expire_at", time.Now().UTC().Add(-time.Minute))
	if ok, _ := repo.CloseIfExpired(ctx, "ORD-T", time.Now().UTC()); !ok {
		t.Fatal("过期订单应被关闭")
	}
	if ok, _ := repo.CloseIfExpired(ctx, "ORD-T", time.Now().UTC()); ok {
		t.Fatal("重复关闭应返回 false")
	}
	// 未过期订单不可关闭
	_ = repo.Create(ctx, newOrder("ORD-LIVE"))
	if ok, _ := repo.CloseIfExpired(ctx, "ORD-LIVE", time.Now().UTC()); ok {
		t.Fatal("未过期订单不应被关闭")
	}
}

// Transition 条件更新：仅 from 状态匹配时生效。
func TestTransition(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-TR"))
	now := time.Now().UTC()
	ok, err := repo.Transition(ctx, "ORD-TR", entity.StatusPendingPayment, entity.StatusPaid, nil, now)
	if err != nil || !ok {
		t.Fatalf("transition: %v / %v", ok, err)
	}
	ok, _ = repo.Transition(ctx, "ORD-TR", entity.StatusPendingPayment, entity.StatusCancelled, nil, now)
	if ok {
		t.Fatal("状态不匹配时 transition 应返回 false")
	}
	if _, err := repo.Transition(ctx, "ORD-NONE", entity.StatusPendingPayment, entity.StatusPaid, nil, now); err != entity.ErrOrderNotFound {
		t.Fatalf("未知订单 transition 应 ErrOrderNotFound, got %v", err)
	}
}

// 列表与记录。
func TestListsAndRecords(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-L1"))
	_ = repo.Create(ctx, newOrder("ORD-L2"))
	orders, total, err := repo.ListByBuyer(ctx, "buyer1", 0, 20)
	if err != nil || total != 2 || len(orders) != 2 {
		t.Fatalf("ListByBuyer: total = %d, err = %v", total, err)
	}
	paid := newOrder("ORD-P1")
	paid.Status = entity.StatusPaid
	_ = repo.Create(ctx, paid)
	shippable, _ := repo.ListByStatus(ctx, entity.StatusPaid)
	if len(shippable) != 1 || shippable[0].OrderNo != "ORD-P1" {
		t.Fatalf("ListByStatus = %+v", shippable)
	}
	if err := repo.CreateRecord(ctx, &entity.OrderRecord{OrderNo: "ORD-L1", Type: entity.RecordTypePay, Operator: "channel", Detail: "支付成功"}); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	records, _ := repo.ListRecords(ctx, "ORD-L1")
	if len(records) != 1 || records[0].Type != entity.RecordTypePay {
		t.Fatalf("records = %+v", records)
	}
}

// 退款申请与异常支付存储。
func TestRefundsAndExceptions(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	_ = repo.Create(ctx, newOrder("ORD-R"))
	req := &entity.RefundRequest{OrderNo: "ORD-R", BuyerID: "buyer1", Reason: "不想要", Status: entity.RefundPending}
	if err := repo.CreateRefundRequest(ctx, req); err != nil {
		t.Fatalf("CreateRefundRequest: %v", err)
	}
	if req.ID == 0 {
		t.Fatal("应分配退款申请 ID")
	}
	got, _ := repo.FindPendingRefundByOrder(ctx, "ORD-R")
	if got == nil || got.Reason != "不想要" {
		t.Fatalf("FindPendingRefundByOrder = %+v", got)
	}
	latest, _ := repo.FindLatestRefundByOrder(ctx, "ORD-R")
	if latest == nil || latest.ID != req.ID {
		t.Fatalf("FindLatestRefundByOrder = %+v", latest)
	}
	if latestNone, _ := repo.FindLatestRefundByOrder(ctx, "ORD-NONE"); latestNone != nil {
		t.Fatal("无退款申请应返回 nil")
	}
	got.Status = entity.RefundApproved
	if err := repo.UpdateRefundRequest(ctx, got); err != nil {
		t.Fatalf("UpdateRefundRequest: %v", err)
	}
	updated, _ := repo.FindRefundRequest(ctx, req.ID)
	if updated.Status != entity.RefundApproved {
		t.Fatalf("更新后状态 = %s", updated.Status)
	}

	ex := &entity.PayException{OrderNo: "ORD-R", ChannelTradeNo: "CH-LATE", AmountCents: 10000, Reason: "late_payment", Status: entity.ExceptionPending}
	if err := repo.CreatePayException(ctx, ex); err != nil {
		t.Fatalf("CreatePayException: %v", err)
	}
	byTrade, _ := repo.FindPayExceptionByTradeNo(ctx, "CH-LATE")
	if byTrade == nil {
		t.Fatal("按交易号查询异常应命中")
	}
	if none, _ := repo.FindPayExceptionByTradeNo(ctx, "CH-NONE"); none != nil {
		t.Fatal("未知交易号应返回 nil")
	}
	ex.Status = entity.ExceptionRefunded
	if err := repo.UpdatePayException(ctx, ex); err != nil {
		t.Fatalf("UpdatePayException: %v", err)
	}
	pending, _ := repo.ListPayExceptions(ctx, entity.ExceptionPending)
	if len(pending) != 0 {
		t.Fatalf("关闭后 pending 异常应为空, got %d", len(pending))
	}
}
