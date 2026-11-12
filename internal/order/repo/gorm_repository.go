package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

// GormRepository 使用 Gorm + 事务内条件更新实现订单域仓储。
type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&dbmodel.OrderModel{},
		&dbmodel.OrderRecordModel{},
		&dbmodel.RefundRequestModel{},
		&dbmodel.PayExceptionModel{},
	)
}

func (r *GormRepository) Create(ctx context.Context, order *entity.Order) error {
	record := toOrderModel(order)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return err
	}
	return nil
}

func (r *GormRepository) FindByOrderNo(ctx context.Context, orderNo string) (*entity.Order, error) {
	var record dbmodel.OrderModel
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return toOrderDomain(&record), nil
}

// CancelPending 条件更新：仅 PENDING_PAYMENT 可取消，重复取消幂等返回 false。
func (r *GormRepository) CancelPending(ctx context.Context, orderNo string, reason string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).
		Where("order_no = ? AND status = ?", orderNo, entity.StatusPendingPayment).
		Updates(map[string]interface{}{
			"status":        entity.StatusCancelled,
			"cancel_reason": reason,
			"closed_at":     now.UTC(),
			"updated_at":    now.UTC(),
		})
	return res.RowsAffected == 1, res.Error
}

// CloseIfExpired 条件更新：仅已过期待付款订单会被关闭（服务端时间判断），天然幂等。
func (r *GormRepository) CloseIfExpired(ctx context.Context, orderNo string, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).
		Where("order_no = ? AND status = ? AND pay_expire_at < ?", orderNo, entity.StatusPendingPayment, now.UTC()).
		Updates(map[string]interface{}{
			"status":     entity.StatusClosedTimeout,
			"closed_at":  now.UTC(),
			"updated_at": now.UTC(),
		})
	return res.RowsAffected == 1, res.Error
}

// MarkPaid 条件更新：仅未过期的 PENDING_PAYMENT 订单可被支付成功通知标记为 PAID。
// channel_trade_no 唯一索引兜底“同一渠道交易号只归属一个订单”。
func (r *GormRepository) MarkPaid(ctx context.Context, orderNo string, tradeNo string, now time.Time) (bool, error) {
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPaymentFacts(tx); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&dbmodel.PayExceptionModel{}).Where("channel_trade_no = ? AND reason = ? AND order_no <> ?", tradeNo, "late_payment", orderNo).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return entity.ErrTradeNoConflict
		}
		res := tx.Model(&dbmodel.OrderModel{}).
			Where("order_no = ? AND status = ? AND pay_expire_at >= ?", orderNo, entity.StatusPendingPayment, now.UTC()).
			Updates(map[string]interface{}{"status": entity.StatusPaid, "channel_trade_no": tradeNo, "paid_at": now.UTC(), "updated_at": now.UTC()})
		changed = res.RowsAffected == 1
		return res.Error
	})
	return changed, err
}

// Transition 条件更新 from -> to，apply 在事务内修改字段。
func (r *GormRepository) Transition(ctx context.Context, orderNo string, from string, to string, apply func(*entity.Order), now time.Time) (bool, error) {
	var updated bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPaymentFacts(tx); err != nil {
			return err
		}
		var record dbmodel.OrderModel
		err := tx.Where("order_no = ?", orderNo).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if record.Status != from {
			updated = false
			return nil
		}
		if apply != nil {
			order := toOrderDomain(&record)
			apply(order)
			applyToModel(order, &record)
		}
		record.Status = to
		record.UpdatedAt = now.UTC()
		res := tx.Model(&dbmodel.OrderModel{}).
			Where("order_no = ? AND status = ?", orderNo, from).
			Select("status", "updated_at", "paid_at", "carrier", "tracking_no", "shipped_at", "cancel_reason", "closed_at", "refund_fail_reason", "refunded_at").
			Updates(&record)
		updated = res.RowsAffected == 1
		return res.Error
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

func (r *GormRepository) ListByBuyer(ctx context.Context, buyerID string, offset int, limit int) ([]*entity.Order, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).Where("buyer_id = ?", buyerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []dbmodel.OrderModel
	if err := r.db.WithContext(ctx).Where("buyer_id = ?", buyerID).Order("id DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*entity.Order, 0, len(records))
	for i := range records {
		items = append(items, toOrderDomain(&records[i]))
	}
	return items, total, nil
}

func (r *GormRepository) ListByStatus(ctx context.Context, status string) ([]*entity.Order, error) {
	var records []dbmodel.OrderModel
	if err := r.db.WithContext(ctx).Where("status = ?", status).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.Order, 0, len(records))
	for i := range records {
		items = append(items, toOrderDomain(&records[i]))
	}
	return items, nil
}

func (r *GormRepository) ListExpiredPending(ctx context.Context, now time.Time) ([]*entity.Order, error) {
	var records []dbmodel.OrderModel
	if err := r.db.WithContext(ctx).
		Where("status = ? AND pay_expire_at < ?", entity.StatusPendingPayment, now.UTC()).
		Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.Order, 0, len(records))
	for i := range records {
		items = append(items, toOrderDomain(&records[i]))
	}
	return items, nil
}

func (r *GormRepository) CreateRecord(ctx context.Context, record *entity.OrderRecord) error {
	m := &dbmodel.OrderRecordModel{
		OrderNo:  record.OrderNo,
		Type:     record.Type,
		Operator: record.Operator,
		Detail:   record.Detail,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	record.ID = m.ID
	record.CreatedAt = m.CreatedAt
	return nil
}

func (r *GormRepository) ListRecords(ctx context.Context, orderNo string) ([]*entity.OrderRecord, error) {
	var records []dbmodel.OrderRecordModel
	if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.OrderRecord, 0, len(records))
	for i := range records {
		m := &records[i]
		items = append(items, &entity.OrderRecord{
			ID:        m.ID,
			OrderNo:   m.OrderNo,
			Type:      m.Type,
			Operator:  m.Operator,
			Detail:    m.Detail,
			CreatedAt: m.CreatedAt,
		})
	}
	return items, nil
}

func (r *GormRepository) CreateRefundRequest(ctx context.Context, req *entity.RefundRequest) error {
	m := &dbmodel.RefundRequestModel{
		OrderNo:   req.OrderNo,
		BuyerID:   req.BuyerID,
		Reason:    req.Reason,
		Status:    req.Status,
		CreatedAt: req.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	req.ID = m.ID
	req.CreatedAt = m.CreatedAt
	return nil
}

func (r *GormRepository) FindRefundRequest(ctx context.Context, id uint) (*entity.RefundRequest, error) {
	var m dbmodel.RefundRequestModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toRefundDomain(&m), nil
}

func (r *GormRepository) FindPendingRefundByOrder(ctx context.Context, orderNo string) (*entity.RefundRequest, error) {
	var m dbmodel.RefundRequestModel
	err := r.db.WithContext(ctx).
		Where("order_no = ? AND status = ?", orderNo, entity.RefundPending).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRefundDomain(&m), nil
}

func (r *GormRepository) FindLatestRefundByOrder(ctx context.Context, orderNo string) (*entity.RefundRequest, error) {
	var m dbmodel.RefundRequestModel
	err := r.db.WithContext(ctx).
		Where("order_no = ?", orderNo).
		Order("id DESC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRefundDomain(&m), nil
}

func (r *GormRepository) UpdateRefundRequest(ctx context.Context, req *entity.RefundRequest) error {
	m := &dbmodel.RefundRequestModel{
		ID:           req.ID,
		OrderNo:      req.OrderNo,
		BuyerID:      req.BuyerID,
		Reason:       req.Reason,
		Status:       req.Status,
		ReviewerID:   req.ReviewerID,
		ReviewReason: req.ReviewReason,
		CreatedAt:    req.CreatedAt,
		ReviewedAt:   req.ReviewedAt,
	}
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *GormRepository) ListRefundRequests(ctx context.Context, status string) ([]*entity.RefundRequest, error) {
	query := r.db.WithContext(ctx).Model(&dbmodel.RefundRequestModel{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var records []dbmodel.RefundRequestModel
	if err := query.Order("id DESC").Limit(200).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.RefundRequest, 0, len(records))
	for i := range records {
		items = append(items, toRefundDomain(&records[i]))
	}
	return items, nil
}

func (r *GormRepository) CreatePayException(ctx context.Context, ex *entity.PayException) error {
	m := &dbmodel.PayExceptionModel{
		OrderNo:        ex.OrderNo,
		ChannelTradeNo: ex.ChannelTradeNo,
		AmountCents:    ex.AmountCents,
		Reason:         ex.Reason,
		Status:         ex.Status,
		Detail:         ex.Detail,
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPaymentFacts(tx); err != nil {
			return err
		}
		if ex.Reason == "late_payment" {
			var count int64
			if err := tx.Model(&dbmodel.OrderModel{}).Where("channel_trade_no = ? AND order_no <> ?", ex.ChannelTradeNo, ex.OrderNo).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return entity.ErrTradeNoConflict
			}
			var existing dbmodel.PayExceptionModel
			err := tx.Where("channel_trade_no = ?", ex.ChannelTradeNo).First(&existing).Error
			if err == nil {
				if existing.Reason == "late_payment" {
					if existing.OrderNo != ex.OrderNo || existing.AmountCents != ex.AmountCents {
						return entity.ErrTradeNoConflict
					}
					*m = existing
					return nil
				}
				m.ID = existing.ID
				m.CreatedAt = existing.CreatedAt
				m.Detail = existing.Detail + "；合法成功通知纠正：" + m.Detail
				return tx.Save(m).Error
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Create(m).Error
	}); err != nil {
		return err
	}
	ex.ID = m.ID
	ex.CreatedAt = m.CreatedAt
	return nil
}

func (r *GormRepository) FindPayException(ctx context.Context, id uint) (*entity.PayException, error) {
	var m dbmodel.PayExceptionModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return toExceptionDomain(&m), nil
}

func (r *GormRepository) FindPayExceptionByTradeNo(ctx context.Context, tradeNo string) (*entity.PayException, error) {
	var m dbmodel.PayExceptionModel
	err := r.db.WithContext(ctx).Where("channel_trade_no = ?", tradeNo).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toExceptionDomain(&m), nil
}

func (r *GormRepository) UpdatePayException(ctx context.Context, ex *entity.PayException) error {
	m := &dbmodel.PayExceptionModel{
		ID:             ex.ID,
		OrderNo:        ex.OrderNo,
		ChannelTradeNo: ex.ChannelTradeNo,
		AmountCents:    ex.AmountCents,
		Reason:         ex.Reason,
		Status:         ex.Status,
		Detail:         ex.Detail,
		CreatedAt:      ex.CreatedAt,
		HandledAt:      ex.HandledAt,
	}
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *GormRepository) ListPayExceptions(ctx context.Context, status string) ([]*entity.PayException, error) {
	query := r.db.WithContext(ctx).Model(&dbmodel.PayExceptionModel{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var records []dbmodel.PayExceptionModel
	if err := query.Order("id DESC").Limit(200).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.PayException, 0, len(records))
	for i := range records {
		items = append(items, toExceptionDomain(&records[i]))
	}
	return items, nil
}

// Summary 运营汇总：已收款、已退款（含异常支付退款）、各状态订单数、异常数。
func (r *GormRepository) Summary(ctx context.Context) (int64, int64, map[string]int64, int64, error) {
	var paidCents, refundedCents, exceptionCount int64

	err := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).
		Where("channel_trade_no <> '' AND status IN ?", []string{
			entity.StatusPaid, entity.StatusShipped, entity.StatusRefundRequested,
			entity.StatusRefundFailed, entity.StatusRefunded,
		}).
		Select("COALESCE(SUM(unit_price_cents * quantity), 0)").Scan(&paidCents).Error
	if err != nil {
		return 0, 0, nil, 0, err
	}

	err = r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).
		Where("status = ?", entity.StatusRefunded).
		Select("COALESCE(SUM(unit_price_cents * quantity), 0)").Scan(&refundedCents).Error
	if err != nil {
		return 0, 0, nil, 0, err
	}

	// 迟到的真实成功支付也是收款事实，不能只累计其退款。
	var latePaid int64
	if err := r.db.WithContext(ctx).Model(&dbmodel.PayExceptionModel{}).
		Where("reason = ?", "late_payment").
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&latePaid).Error; err != nil {
		return 0, 0, nil, 0, err
	}
	paidCents += latePaid

	// 迟到支付退款单独累计
	var exceptionRefunded int64
	err = r.db.WithContext(ctx).Model(&dbmodel.PayExceptionModel{}).
		Where("status = ? AND reason = ?", entity.ExceptionRefunded, "late_payment").
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&exceptionRefunded).Error
	if err != nil {
		return 0, 0, nil, 0, err
	}
	refundedCents += exceptionRefunded

	err = r.db.WithContext(ctx).Model(&dbmodel.PayExceptionModel{}).Count(&exceptionCount).Error
	if err != nil {
		return 0, 0, nil, 0, err
	}

	type statusCount struct {
		Status string
		Count  int64
	}
	var countsRaw []statusCount
	if err := r.db.WithContext(ctx).Model(&dbmodel.OrderModel{}).
		Select("status, COUNT(*) as count").Group("status").Scan(&countsRaw).Error; err != nil {
		return 0, 0, nil, 0, err
	}
	counts := make(map[string]int64, len(countsRaw))
	for _, item := range countsRaw {
		counts[item.Status] = item.Count
	}
	return paidCents, refundedCents, counts, exceptionCount, nil
}

func toOrderModel(order *entity.Order) *dbmodel.OrderModel {
	return &dbmodel.OrderModel{
		OrderNo:          order.OrderNo,
		BuyerID:          order.BuyerID,
		ProductID:        order.ProductID,
		ProductName:      order.ProductName,
		UnitPriceCents:   order.UnitPriceCents,
		Quantity:         order.Quantity,
		Status:           order.Status,
		ChannelTradeNo:   tradeNoPtr(order.ChannelTradeNo),
		PaidAt:           order.PaidAt,
		Carrier:          order.Carrier,
		TrackingNo:       order.TrackingNo,
		ShippedAt:        order.ShippedAt,
		CancelReason:     order.CancelReason,
		ClosedAt:         order.ClosedAt,
		RefundFailReason: order.RefundFailReason,
		RefundedAt:       order.RefundedAt,
		PayExpireAt:      order.PayExpireAt,
		CreatedAt:        order.CreatedAt,
		UpdatedAt:        order.UpdatedAt,
	}
}

func applyToModel(order *entity.Order, record *dbmodel.OrderModel) {
	record.PaidAt = order.PaidAt
	record.Carrier = order.Carrier
	record.TrackingNo = order.TrackingNo
	record.ShippedAt = order.ShippedAt
	record.CancelReason = order.CancelReason
	record.ClosedAt = order.ClosedAt
	record.RefundFailReason = order.RefundFailReason
	record.RefundedAt = order.RefundedAt
}

func tradeNoPtr(value string) *string {
	if value == `` {
		return nil
	}
	return &value
}

func tradeNoValue(ptr *string) string {
	if ptr == nil {
		return ``
	}
	return *ptr
}

func toOrderDomain(record *dbmodel.OrderModel) *entity.Order {
	return &entity.Order{
		OrderNo:          record.OrderNo,
		BuyerID:          record.BuyerID,
		ProductID:        record.ProductID,
		ProductName:      record.ProductName,
		UnitPriceCents:   record.UnitPriceCents,
		Quantity:         record.Quantity,
		Status:           record.Status,
		ChannelTradeNo:   tradeNoValue(record.ChannelTradeNo),
		PaidAt:           record.PaidAt,
		Carrier:          record.Carrier,
		TrackingNo:       record.TrackingNo,
		ShippedAt:        record.ShippedAt,
		CancelReason:     record.CancelReason,
		ClosedAt:         record.ClosedAt,
		RefundFailReason: record.RefundFailReason,
		RefundedAt:       record.RefundedAt,
		PayExpireAt:      record.PayExpireAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

func toRefundDomain(m *dbmodel.RefundRequestModel) *entity.RefundRequest {
	return &entity.RefundRequest{
		ID:           m.ID,
		OrderNo:      m.OrderNo,
		BuyerID:      m.BuyerID,
		Reason:       m.Reason,
		Status:       m.Status,
		ReviewerID:   m.ReviewerID,
		ReviewReason: m.ReviewReason,
		CreatedAt:    m.CreatedAt,
		ReviewedAt:   m.ReviewedAt,
	}
}

func toExceptionDomain(m *dbmodel.PayExceptionModel) *entity.PayException {
	return &entity.PayException{
		ID:             m.ID,
		OrderNo:        m.OrderNo,
		ChannelTradeNo: m.ChannelTradeNo,
		AmountCents:    m.AmountCents,
		Reason:         m.Reason,
		Status:         m.Status,
		Detail:         m.Detail,
		CreatedAt:      m.CreatedAt,
		HandledAt:      m.HandledAt,
	}
}

var _ entity.Repository = (*GormRepository)(nil)

// lockPaymentFacts obtains SQLite's database writer reservation before reading either fact table.
// The teaching deployment uses SQLite; serializable cross-table facts must be revisited before changing databases.
func lockPaymentFacts(tx *gorm.DB) error {
	return tx.Exec("UPDATE orders SET updated_at = updated_at WHERE 1 = 0").Error
}

// ApplyRefundAtomic commits the state transition and its application together.
func (r *GormRepository) ApplyRefundAtomic(ctx context.Context, orderNo, buyerID, reason string, now time.Time) (bool, error) {
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&dbmodel.OrderModel{}).Where("order_no = ? AND buyer_id = ? AND status = ?", orderNo, buyerID, entity.StatusPaid).Updates(map[string]interface{}{"status": entity.StatusRefundRequested, "updated_at": now.UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Create(&dbmodel.RefundRequestModel{OrderNo: orderNo, BuyerID: buyerID, Reason: reason, Status: entity.RefundPending, CreatedAt: now.UTC()}).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

// ReviewRefundAtomic CASes the review and, when rejected, restores shipping in the same transaction.
func (r *GormRepository) ReviewRefundAtomic(ctx context.Context, req *entity.RefundRequest, approve bool, now time.Time) (bool, error) {
	var changed bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		status := entity.RefundRejected
		if approve {
			status = entity.RefundApproved
		}
		res := tx.Model(&dbmodel.RefundRequestModel{}).Where("id = ? AND order_no = ? AND status = ?", req.ID, req.OrderNo, entity.RefundPending).Updates(map[string]interface{}{"status": status, "reviewer_id": req.ReviewerID, "review_reason": req.ReviewReason, "reviewed_at": now.UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		target := entity.StatusPaid
		if approve {
			target = entity.StatusRefundRequested
		}
		state := tx.Model(&dbmodel.OrderModel{}).Where("order_no = ? AND status = ?", req.OrderNo, entity.StatusRefundRequested).Updates(map[string]interface{}{"status": target, "updated_at": now.UTC()})
		if state.Error != nil {
			return state.Error
		}
		if state.RowsAffected != 1 {
			return entity.ErrInvalidState
		}
		changed = true
		return nil
	})
	return changed, err
}
