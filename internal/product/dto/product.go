package dto

import "github.com/BwCloudWeGo/bw-cli/internal/product/entity"

// ProductDTO 是商品用例层返回的数据结构。
type ProductDTO struct {
	ID         string
	Name       string
	PriceCents int64
	StockTotal int64
	Reserved   int64
	Available  int64
	OnSale     bool
	UpdatedAt  string
}

func FromProduct(product *entity.Product) *ProductDTO {
	return &ProductDTO{
		ID:         product.ID,
		Name:       product.Name,
		PriceCents: product.PriceCents,
		StockTotal: product.StockTotal,
		Reserved:   product.Reserved,
		Available:  product.Available(),
		OnSale:     product.OnSale,
		UpdatedAt:  product.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func FromProducts(products []*entity.Product) []*ProductDTO {
	items := make([]*ProductDTO, 0, len(products))
	for _, item := range products {
		items = append(items, FromProduct(item))
	}
	return items
}

// CreateCommand 运营创建商品。
type CreateCommand struct {
	Name       string
	PriceCents int64
	StockTotal int64
}

// UpdateCommand 运营调整商品：售价、上下架、补货。
type UpdateCommand struct {
	ID         string
	SetPrice   bool
	PriceCents int64
	SetOnSale  bool
	OnSale     bool
	StockDelta int64
}
