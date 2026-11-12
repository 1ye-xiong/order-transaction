package model

import "time"

// ProductModel 是 products 表的 Gorm 持久化模型。
type ProductModel struct {
	ID         string `gorm:"primaryKey;column:id;size:32"`
	Name       string `gorm:"column:name;size:128;not null"`
	PriceCents int64  `gorm:"column:price_cents;not null"`
	StockTotal int64  `gorm:"column:stock_total;not null"`
	Reserved   int64  `gorm:"column:reserved;not null;default:0"`
	OnSale     bool   `gorm:"column:on_sale;not null;default:true"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ProductModel) TableName() string {
	return "products"
}

// ReservationModel 是 product_reservations 表：一个订单对一种商品的库存占用。
// order_no 唯一，保证并发下单时同一订单不会重复占用。
type ReservationModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	OrderNo   string `gorm:"uniqueIndex;column:order_no;size:40;not null"`
	ProductID string `gorm:"column:product_id;size:32;not null"`
	Quantity  int64  `gorm:"column:quantity;not null"`
	Status    string `gorm:"column:status;size:16;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ReservationModel) TableName() string {
	return "product_reservations"
}
