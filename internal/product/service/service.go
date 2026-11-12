package service

import (
	"context"

	"github.com/BwCloudWeGo/bw-cli/internal/product/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/product/entity"
)

// Service 编排商品用例。
type Service struct {
	repo entity.Repository
}

func NewService(repo entity.Repository) *Service {
	return &Service{repo: repo}
}

// Create 创建商品（运营）。
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) (*dto.ProductDTO, error) {
	product, err := entity.NewProduct(cmd.Name, cmd.PriceCents, cmd.StockTotal)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, product); err != nil {
		return nil, err
	}
	return dto.FromProduct(product), nil
}

// Update 调整售价、上下架或补货（运营）。已有订单金额不受调价影响。
func (s *Service) Update(ctx context.Context, cmd dto.UpdateCommand) (*dto.ProductDTO, error) {
	if atomic, ok := s.repo.(interface {
		UpdateAtomic(context.Context, dto.UpdateCommand) (*entity.Product, error)
	}); ok {
		product, err := atomic.UpdateAtomic(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return dto.FromProduct(product), nil
	}
	product, err := s.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if cmd.SetPrice {
		if err := product.ChangePrice(cmd.PriceCents); err != nil {
			return nil, err
		}
	}
	if cmd.SetOnSale {
		product.SetOnSale(cmd.OnSale)
	}
	if cmd.StockDelta != 0 {
		if err := product.Restock(cmd.StockDelta); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Save(ctx, product); err != nil {
		return nil, err
	}
	return dto.FromProduct(product), nil
}

// List 商品列表；买家视角只看在售商品。
func (s *Service) List(ctx context.Context, includeOffSale bool) ([]*dto.ProductDTO, error) {
	products, err := s.repo.List(ctx, includeOffSale)
	if err != nil {
		return nil, err
	}
	return dto.FromProducts(products), nil
}

// GetProduct 查询单个商品。
func (s *Service) GetProduct(ctx context.Context, id string) (*dto.ProductDTO, error) {
	product, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.FromProduct(product), nil
}

// Reserve 为订单占用库存（订单服务内部调用）。
func (s *Service) Reserve(ctx context.Context, orderNo string, productID string, quantity int64) error {
	return s.repo.Reserve(ctx, orderNo, productID, quantity)
}

// Release 释放订单占用（订单关闭/取消，幂等）。
func (s *Service) Release(ctx context.Context, orderNo string) error {
	return s.repo.Release(ctx, orderNo)
}

// Confirm 支付成功后确认扣减库存（幂等）。
func (s *Service) Confirm(ctx context.Context, orderNo string) error {
	return s.repo.Confirm(ctx, orderNo)
}

// Restore 售前退款成功后恢复库存（幂等）。
func (s *Service) Restore(ctx context.Context, orderNo string) error {
	return s.repo.Restore(ctx, orderNo)
}
