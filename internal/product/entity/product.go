package entity

import (
	"context"
	"errors"
	"time"
)

var (
	ErrProductNotFound    = errors.New("product not found")
	ErrInvalidProduct     = errors.New("invalid product")
	ErrOffSale            = errors.New("product off sale")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrInvalidQuantity    = errors.New("invalid quantity")
	ErrReservationMissing = errors.New("reservation missing")
)

const (
	ReservationReserved  = "reserved"
	ReservationConfirmed = "confirmed"
	ReservationReleased  = "released"
	ReservationRestored  = "restored"
)

// Product 是商品聚合。可售数量 = StockTotal - Reserved，反映已占用数量。
type Product struct {
	ID         string
	Name       string
	PriceCents int64
	StockTotal int64
	Reserved   int64
	OnSale     bool
	UpdatedAt  time.Time
}

// Available 返回当前可售数量。
func (p *Product) Available() int64 {
	return p.StockTotal - p.Reserved
}

// NewProduct 校验并创建商品聚合。
func NewProduct(name string, priceCents int64, stockTotal int64) (*Product, error) {
	if len(name) == 0 || priceCents <= 0 || stockTotal < 0 {
		return nil, ErrInvalidProduct
	}
	return &Product{
		Name:       name,
		PriceCents: priceCents,
		StockTotal: stockTotal,
		OnSale:     true,
		UpdatedAt:  time.Now().UTC(),
	}, nil
}

// ChangePrice 调整售价。已有订单金额不受影响（订单保存下单时价格）。
func (p *Product) ChangePrice(priceCents int64) error {
	if priceCents <= 0 {
		return ErrInvalidProduct
	}
	p.PriceCents = priceCents
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Restock 补货，仅允许增加总量。
func (p *Product) Restock(delta int64) error {
	if delta <= 0 {
		return ErrInvalidProduct
	}
	p.StockTotal += delta
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// SetOnSale 上下架。
func (p *Product) SetOnSale(onSale bool) {
	p.OnSale = onSale
	p.UpdatedAt = time.Now().UTC()
}

// Reservation 记录一个订单对商品的库存占用，用于幂等释放与确认。
type Reservation struct {
	OrderNo   string
	ProductID string
	Quantity  int64
	Status    string
}

// Repository 是商品服务的仓储接口。
type Repository interface {
	Save(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id string) (*Product, error)
	List(ctx context.Context, includeOffSale bool) ([]*Product, error)
	Reserve(ctx context.Context, orderNo string, productID string, quantity int64) error
	Release(ctx context.Context, orderNo string) error
	Confirm(ctx context.Context, orderNo string) error
	// Restore 把已确认扣减的库存恢复为可售（售前退款成功）；重复恢复幂等。
	Restore(ctx context.Context, orderNo string) error
	FindReservation(ctx context.Context, orderNo string) (*Reservation, error)
}
