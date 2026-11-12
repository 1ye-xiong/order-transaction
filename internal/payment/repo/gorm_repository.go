package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/payment/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/payment/model"
)

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.ChannelConfigModel{}, &dbmodel.ChannelRecordModel{})
}

func (r *GormRepository) SaveConfig(ctx context.Context, config entity.ChannelConfig) error {
	config.Normalize()
	var record dbmodel.ChannelConfigModel
	err := r.db.WithContext(ctx).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = dbmodel.ChannelConfigModel{
			PayBehavior:              config.PayBehavior,
			NotifyCount:              config.NotifyCount,
			NotifyDelaySeconds:       config.NotifyDelaySeconds,
			PayLateSeconds:           config.PayLateSeconds,
			RefundSuccessFromAttempt: config.RefundSuccessFromAttempt,
		}
		return r.db.WithContext(ctx).Create(&record).Error
	}
	if err != nil {
		return err
	}
	record.PayBehavior = config.PayBehavior
	record.NotifyCount = config.NotifyCount
	record.NotifyDelaySeconds = config.NotifyDelaySeconds
	record.PayLateSeconds = config.PayLateSeconds
	record.RefundSuccessFromAttempt = config.RefundSuccessFromAttempt
	return r.db.WithContext(ctx).Save(&record).Error
}

func (r *GormRepository) LoadConfig(ctx context.Context) (entity.ChannelConfig, error) {
	var record dbmodel.ChannelConfigModel
	err := r.db.WithContext(ctx).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entity.DefaultChannelConfig(), nil
	}
	if err != nil {
		return entity.ChannelConfig{}, err
	}
	return entity.ChannelConfig{
		PayBehavior:              record.PayBehavior,
		NotifyCount:              record.NotifyCount,
		NotifyDelaySeconds:       record.NotifyDelaySeconds,
		PayLateSeconds:           record.PayLateSeconds,
		RefundSuccessFromAttempt: record.RefundSuccessFromAttempt,
	}, nil
}

func (r *GormRepository) SaveRecord(ctx context.Context, record *entity.ChannelRecord) error {
	m := &dbmodel.ChannelRecordModel{
		Type:           record.Type,
		ChannelTradeNo: record.ChannelTradeNo,
		RefundNo:       record.RefundNo,
		OrderNo:        record.OrderNo,
		AmountCents:    record.AmountCents,
		Result:         record.Result,
		Attempts:       record.Attempts,
		Detail:         record.Detail,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	record.ID = m.ID
	record.CreatedAt = m.CreatedAt
	return nil
}

func (r *GormRepository) ListRecords(ctx context.Context) ([]*entity.ChannelRecord, error) {
	var records []dbmodel.ChannelRecordModel
	if err := r.db.WithContext(ctx).Order("id DESC").Limit(200).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.ChannelRecord, 0, len(records))
	for i := range records {
		m := &records[i]
		items = append(items, &entity.ChannelRecord{
			ID:             m.ID,
			Type:           m.Type,
			ChannelTradeNo: m.ChannelTradeNo,
			RefundNo:       m.RefundNo,
			OrderNo:        m.OrderNo,
			AmountCents:    m.AmountCents,
			Result:         m.Result,
			Attempts:       m.Attempts,
			Detail:         m.Detail,
			CreatedAt:      m.CreatedAt,
		})
	}
	return items, nil
}

// CountRefundAttempts 统计同一退款单的历史尝试次数，用于"第 N 次起成功"。
func (r *GormRepository) CountRefundAttempts(ctx context.Context, refundNo string) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&dbmodel.ChannelRecordModel{}).
		Where("type = ? AND refund_no = ?", entity.RecordTypeRefund, refundNo).
		Count(&count).Error
	return int(count), err
}

// ExecuteRefund atomically verifies the original receipt, prevents a second
// successful refund and persists each failed attempt under the same business ID.
func (r *GormRepository) ExecuteRefund(ctx context.Context, refundNo, tradeNo string, amount int64, reason string) (bool, string, error) {
	if refundNo == "" || tradeNo == "" || amount <= 0 {
		return false, "", errors.New("invalid refund")
	}
	var success bool
	var message string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire the database write lock before reading facts (also works on SQLite).
		if err := tx.Model(&dbmodel.ChannelRecordModel{}).Where("channel_trade_no = ?", tradeNo).UpdateColumn("attempts", gorm.Expr("attempts")).Error; err != nil {
			return err
		}
		var payment dbmodel.ChannelRecordModel
		if err := tx.Where("type = ? AND channel_trade_no = ? AND result = ?", entity.RecordTypePayment, tradeNo, "success").First(&payment).Error; err != nil {
			return errors.New("confirmed payment not found")
		}
		if payment.AmountCents != amount {
			return errors.New("refund amount differs from payment")
		}
		var history []dbmodel.ChannelRecordModel
		if err := tx.Where("type = ? AND (channel_trade_no = ? OR refund_no = ?)", entity.RecordTypeRefund, tradeNo, refundNo).Find(&history).Error; err != nil {
			return err
		}
		attempts := 0
		for _, h := range history {
			if h.RefundNo == refundNo && (h.ChannelTradeNo != tradeNo || h.AmountCents != amount) {
				return errors.New("refund identity conflict")
			}
			if h.Result == "success" {
				if h.RefundNo != refundNo {
					return errors.New("payment already refunded")
				}
				success, message = true, "refund already successful"
				return nil
			}
			if h.RefundNo == refundNo {
				attempts++
			}
		}
		var cfg dbmodel.ChannelConfigModel
		err := tx.First(&cfg).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		threshold := cfg.RefundSuccessFromAttempt
		if threshold < 1 {
			threshold = 1
		}
		success = attempts+1 >= threshold
		result := "failed"
		if success {
			result = "success"
		}
		message = "refund " + result
		return tx.Create(&dbmodel.ChannelRecordModel{Type: entity.RecordTypeRefund, ChannelTradeNo: tradeNo, RefundNo: refundNo, OrderNo: payment.OrderNo, AmountCents: amount, Result: result, Attempts: attempts + 1, Detail: reason}).Error
	})
	return success, message, err
}
