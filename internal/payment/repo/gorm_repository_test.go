package repo

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
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

// 配置持久化：保存后重启（重新 Load）可读回。
func TestConfigPersistence(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	config, err := repo.LoadConfig(ctx)
	if err != nil {
		t.Fatalf("首次 LoadConfig 应返回默认值: %v", err)
	}
	if config.PayBehavior != entity.PayBehaviorSuccess {
		t.Fatalf("默认行为 = %s", config.PayBehavior)
	}
	config.PayBehavior = entity.PayBehaviorFail
	config.NotifyCount = 2
	if err := repo.SaveConfig(ctx, config); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	loaded, _ := repo.LoadConfig(ctx)
	if loaded.PayBehavior != entity.PayBehaviorFail || loaded.NotifyCount != 2 {
		t.Fatalf("配置未持久化: %+v", loaded)
	}
	// 二次保存走更新分支
	config.NotifyCount = 3
	if err := repo.SaveConfig(ctx, config); err != nil {
		t.Fatalf("更新配置: %v", err)
	}
	loaded, _ = repo.LoadConfig(ctx)
	if loaded.NotifyCount != 3 {
		t.Fatalf("配置更新失败: %+v", loaded)
	}
}

// 退款尝试计数用于"第 N 次起成功"。
func TestCountRefundAttempts(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_ = repo.SaveRecord(ctx, &entity.ChannelRecord{
			Type: entity.RecordTypeRefund, RefundNo: "RF-1", OrderNo: "ORD-1", AmountCents: 100, Result: "failed",
		})
	}
	count, err := repo.CountRefundAttempts(ctx, "RF-1")
	if err != nil || count != 2 {
		t.Fatalf("attempts = %d, err = %v, want 2", count, err)
	}
	other, _ := repo.CountRefundAttempts(ctx, "RF-2")
	if other != 0 {
		t.Fatalf("其他退款单 attempts = %d, want 0", other)
	}
}

// 流水列表按时间倒序且限量。
func TestListRecords(t *testing.T) {
	repo := NewGormRepository(newTestDB(t))
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = repo.SaveRecord(ctx, &entity.ChannelRecord{
			Type: entity.RecordTypePayment, ChannelTradeNo: "CH", OrderNo: "ORD", AmountCents: 1, Result: "accepted",
		})
	}
	records, err := repo.ListRecords(ctx)
	if err != nil || len(records) != 3 {
		t.Fatalf("records = %d, err = %v", len(records), err)
	}
	if records[0].ID != 3 {
		t.Fatalf("应按 id 倒序, first = %d", records[0].ID)
	}
}
