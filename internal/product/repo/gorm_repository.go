package repo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
	dbmodel "github.com/BwCloudWeGo/bw-cli/internal/product/model"
)

// GormRepository 使用 Gorm 持久化商品聚合与库存占用。
type GormRepository struct {
	db  *gorm.DB
	log *zap.Logger
}

// UpdateAtomic applies an operator update while holding the database write
// transaction. It never writes a stale full aggregate, so a concurrent stock
// reservation cannot be overwritten by a price or sale-status change.
func (r *GormRepository) UpdateAtomic(ctx context.Context, cmd dto.UpdateCommand) (*entity.Product, error) {
	var product *entity.Product
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row dbmodel.ProductModel
		if err := tx.Where("id = ?", cmd.ID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return entity.ErrProductNotFound
			}
			return err
		}
		if cmd.SetPrice {
			if cmd.PriceCents <= 0 {
				return entity.ErrInvalidProduct
			}
			if err := tx.Model(&dbmodel.ProductModel{}).Where("id = ?", cmd.ID).Update("price_cents", cmd.PriceCents).Error; err != nil {
				return err
			}
		}
		if cmd.SetOnSale {
			if err := tx.Model(&dbmodel.ProductModel{}).Where("id = ?", cmd.ID).Update("on_sale", cmd.OnSale).Error; err != nil {
				return err
			}
		}
		if cmd.StockDelta != 0 {
			if cmd.StockDelta < 0 {
				return entity.ErrInvalidProduct
			}
			q := tx.Model(&dbmodel.ProductModel{}).Where("id = ?", cmd.ID).UpdateColumn("stock_total", gorm.Expr("stock_total + ?", cmd.StockDelta))
			if q.Error != nil {
				return q.Error
			}
		}
		var updated dbmodel.ProductModel
		if err := tx.Where("id = ?", cmd.ID).First(&updated).Error; err != nil {
			return err
		}
		product = toProductDomain(&updated)
		return nil
	})
	return product, err
}

func NewGormRepository(db *gorm.DB, loggers ...*zap.Logger) *GormRepository {
	log := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		log = loggers[0]
	}
	return &GormRepository{db: db, log: log}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&dbmodel.ProductModel{}, &dbmodel.ReservationModel{})
}

func (r *GormRepository) Save(ctx context.Context, product *entity.Product) error {
	if product.ID == `` {
		product.ID = newProductID()
	}
	record := toProductModel(product)
	err := r.db.WithContext(ctx).Save(record).Error
	if err != nil {
		return err
	}
	product.ID = record.ID
	return nil
}

func (r *GormRepository) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	var record dbmodel.ProductModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return toProductDomain(&record), nil
}

func (r *GormRepository) List(ctx context.Context, includeOffSale bool) ([]*entity.Product, error) {
	query := r.db.WithContext(ctx).Model(&dbmodel.ProductModel{})
	if !includeOffSale {
		query = query.Where("on_sale = ?", true)
	}
	var records []dbmodel.ProductModel
	if err := query.Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*entity.Product, 0, len(records))
	for i := range records {
		items = append(items, toProductDomain(&records[i]))
	}
	return items, nil
}

// Reserve 在事务内占用库存：
//  1. 同订单已有占用记录则幂等返回；
//  2. 条件更新 products（在售且可售数量足够）保证并发争抢最后一个名额时只有一个成功；
//  3. 插入占用记录（order_no 唯一）。
func (r *GormRepository) Reserve(ctx context.Context, orderNo string, productID string, quantity int64) error {
	if quantity <= 0 {
		return entity.ErrInvalidQuantity
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SQLite deferred transactions must acquire the writer reservation before reading;
		// otherwise concurrent readers cannot upgrade their snapshot and fail SQLITE_BUSY.
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("UPDATE product_reservations SET status = status WHERE 1 = 0").Error; err != nil {
				return err
			}
		}
		var existing dbmodel.ReservationModel
		err := tx.Where("order_no = ?", orderNo).First(&existing).Error
		if err == nil {
			return nil // 幂等：同订单不重复占用
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		res := tx.Exec(
			"UPDATE products SET reserved = reserved + ?, updated_at = ? WHERE id = ? AND on_sale = 1 AND stock_total - reserved >= ?",
			quantity, time.Now().UTC(), productID, quantity,
		)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var product dbmodel.ProductModel
			perr := tx.Where("id = ?", productID).First(&product).Error
			if errors.Is(perr, gorm.ErrRecordNotFound) {
				return entity.ErrProductNotFound
			}
			if perr != nil {
				return perr
			}
			if !product.OnSale {
				return entity.ErrOffSale
			}
			return entity.ErrInsufficientStock
		}

		record := &dbmodel.ReservationModel{
			OrderNo:   orderNo,
			ProductID: productID,
			Quantity:  quantity,
			Status:    entity.ReservationReserved,
		}
		return tx.Create(record).Error
	})
}

// Release 释放订单占用；重复释放幂等。
func (r *GormRepository) Release(ctx context.Context, orderNo string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SQLite deferred transactions must acquire the writer reservation before reading;
		// otherwise concurrent readers cannot upgrade their snapshot and fail SQLITE_BUSY.
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("UPDATE product_reservations SET status = status WHERE 1 = 0").Error; err != nil {
				return err
			}
		}
		var record dbmodel.ReservationModel
		err := tx.Where("order_no = ?", orderNo).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.Status != entity.ReservationReserved {
			return nil // released/confirmed：不多释放
		}
		// 先裁定占用状态，只有本事务获胜才能修改商品数量；失败会整体回滚。
		claim := tx.Model(&dbmodel.ReservationModel{}).
			Where("order_no = ? AND status = ?", orderNo, entity.ReservationReserved).
			Update("status", entity.ReservationReleased)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(
			"UPDATE products SET reserved = MAX(reserved - ?, 0), updated_at = ? WHERE id = ?",
			record.Quantity, time.Now().UTC(), record.ProductID,
		).Error; err != nil {
			return err
		}
		return nil
	})
}

// Confirm 将占用转为实际扣减；重复确认幂等。
func (r *GormRepository) Confirm(ctx context.Context, orderNo string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SQLite deferred transactions must acquire the writer reservation before reading;
		// otherwise concurrent readers cannot upgrade their snapshot and fail SQLITE_BUSY.
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("UPDATE product_reservations SET status = status WHERE 1 = 0").Error; err != nil {
				return err
			}
		}
		var record dbmodel.ReservationModel
		err := tx.Where("order_no = ?", orderNo).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.ErrReservationMissing
		}
		if err != nil {
			return err
		}
		if record.Status != entity.ReservationReserved {
			return nil
		}
		// 先裁定占用状态，只有本事务获胜才能修改商品数量；失败会整体回滚。
		claim := tx.Model(&dbmodel.ReservationModel{}).
			Where("order_no = ? AND status = ?", orderNo, entity.ReservationReserved).
			Update("status", entity.ReservationConfirmed)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(
			"UPDATE products SET stock_total = stock_total - ?, reserved = MAX(reserved - ?, 0), updated_at = ? WHERE id = ?",
			record.Quantity, record.Quantity, time.Now().UTC(), record.ProductID,
		).Error; err != nil {
			return err
		}
		return nil
	})
}

// Restore 把已确认扣减的库存恢复为可售（售前退款成功）；重复恢复幂等。
func (r *GormRepository) Restore(ctx context.Context, orderNo string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// SQLite deferred transactions must acquire the writer reservation before reading;
		// otherwise concurrent readers cannot upgrade their snapshot and fail SQLITE_BUSY.
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("UPDATE product_reservations SET status = status WHERE 1 = 0").Error; err != nil {
				return err
			}
		}
		var record dbmodel.ReservationModel
		err := tx.Where("order_no = ?", orderNo).First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.Status != entity.ReservationConfirmed {
			return nil
		}
		// 先裁定占用状态，只有本事务获胜才能修改商品数量；失败会整体回滚。
		claim := tx.Model(&dbmodel.ReservationModel{}).
			Where("order_no = ? AND status = ?", orderNo, entity.ReservationConfirmed).
			Update("status", entity.ReservationRestored)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(
			"UPDATE products SET stock_total = stock_total + ?, updated_at = ? WHERE id = ?",
			record.Quantity, time.Now().UTC(), record.ProductID,
		).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *GormRepository) FindReservation(ctx context.Context, orderNo string) (*entity.Reservation, error) {
	var record dbmodel.ReservationModel
	err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrReservationMissing
	}
	if err != nil {
		return nil, err
	}
	return &entity.Reservation{
		OrderNo:   record.OrderNo,
		ProductID: record.ProductID,
		Quantity:  record.Quantity,
		Status:    record.Status,
	}, nil
}

func toProductModel(product *entity.Product) *dbmodel.ProductModel {
	return &dbmodel.ProductModel{
		ID:         product.ID,
		Name:       product.Name,
		PriceCents: product.PriceCents,
		StockTotal: product.StockTotal,
		Reserved:   product.Reserved,
		OnSale:     product.OnSale,
		UpdatedAt:  product.UpdatedAt,
	}
}

func toProductDomain(record *dbmodel.ProductModel) *entity.Product {
	return &entity.Product{
		ID:         record.ID,
		Name:       record.Name,
		PriceCents: record.PriceCents,
		StockTotal: record.StockTotal,
		Reserved:   record.Reserved,
		OnSale:     record.OnSale,
		UpdatedAt:  record.UpdatedAt,
	}
}

func newProductID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "P" + hex.EncodeToString(buf)
}
