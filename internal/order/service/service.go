package service

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// ChannelClient 是模拟支付渠道的出站接口。
type ChannelClient interface {
	Pay(ctx context.Context, orderNo string, buyerID string, amountCents int64) (string, error)
	Refund(ctx context.Context, refundNo string, tradeNo string, amountCents int64, reason string) (bool, string, error)
}

// StockClient 是商品服务的出站接口。
type StockClient interface {
	Reserve(ctx context.Context, orderNo string, productID string, quantity int64) error
	Release(ctx context.Context, orderNo string) error
	Confirm(ctx context.Context, orderNo string) error
	Restore(ctx context.Context, orderNo string) error
	GetProduct(ctx context.Context, productID string) (name string, priceCents int64, err error)
}

// Service 编排订单用例。所有状态变更走事务内条件更新，保证并发下的唯一合法结果。
type Service struct {
	repo       entity.Repository
	channel    ChannelClient
	stock      StockClient
	payTimeout time.Duration
	secret     string
	log        *zap.Logger
}

func NewService(repo entity.Repository, channel ChannelClient, stock StockClient, payTimeout time.Duration, secret string, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, channel: channel, stock: stock, payTimeout: payTimeout, secret: secret, log: log}
}

// CreateOrder 买家下单：只能购买在售且数量足够的商品，订单保存下单时商品与价格。
func (s *Service) CreateOrder(ctx context.Context, cmd dto.CreateCommand) (*dto.OrderDTO, error) {
	if cmd.Quantity < 1 || cmd.Quantity > 2 {
		return nil, entity.ErrInvalidQuantity
	}
	name, price, err := s.stock.GetProduct(ctx, cmd.ProductID)
	if err != nil {
		return nil, err
	}
	orderNo, err := newOrderNo()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	order := &entity.Order{
		OrderNo:        orderNo,
		BuyerID:        cmd.BuyerID,
		ProductID:      cmd.ProductID,
		ProductName:    name,
		UnitPriceCents: price,
		Quantity:       cmd.Quantity,
		Status:         entity.StatusPendingPayment,
		PayExpireAt:    now.Add(s.payTimeout),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	// 先占用库存（条件更新防超卖），再落订单。
	if err := s.stock.Reserve(ctx, orderNo, cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, order); err != nil {
		if releaseErr := s.stock.Release(ctx, orderNo); releaseErr != nil {
			return nil, fmt.Errorf("save order: %v; release stock: %w", err, releaseErr)
		}
		return nil, err
	}
	return dto.FromOrder(order), nil
}

// GetOrder 买家查询本人订单，他人订单拒绝访问。
func (s *Service) GetOrder(ctx context.Context, buyerID string, orderNo string) (*dto.OrderDTO, error) {
	order, err := s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != buyerID {
		return nil, entity.ErrNotOrderOwner
	}
	return dto.FromOrder(order), nil
}

// ListMyOrders 买家查询本人订单列表。
func (s *Service) ListMyOrders(ctx context.Context, buyerID string, page int32, pageSize int32) ([]*dto.OrderDTO, int64, error) {
	offset, limit := normalizePagination(page, pageSize)
	orders, total, err := s.repo.ListByBuyer(ctx, buyerID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return dto.FromOrders(orders), total, nil
}

// CancelOrder 买家取消待付款订单；重复取消幂等，不多释放库存。
func (s *Service) CancelOrder(ctx context.Context, buyerID string, orderNo string, reason string) (*dto.OrderDTO, error) {
	order, err := s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != buyerID {
		return nil, entity.ErrNotOrderOwner
	}
	now := time.Now().UTC()
	if order.IsExpired(now) {
		if _, err := s.closeExpired(ctx, order.OrderNo, now); err != nil {
			return nil, err
		}
		order, err = s.repo.FindByOrderNo(ctx, orderNo)
		if err != nil {
			return nil, err
		}
		return dto.FromOrder(order), nil
	}
	changed, err := s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: orderNo, Type: entity.RecordTypeClose, Operator: buyerID, Detail: "买家取消订单：" + reason}, func(r entity.Repository) (bool, error) { return r.CancelPending(ctx, orderNo, reason, now) })
	if err != nil {
		return nil, err
	}
	if changed || order.Status == entity.StatusCancelled || order.Status == entity.StatusClosedTimeout {
		if err := s.syncStock(ctx, orderNo, "release"); err != nil {
			return nil, err
		}
	}
	order, err = s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	return dto.FromOrder(order), nil
}

// PayOrder 买家对待付款订单发起支付，结果由渠道异步通知。
func (s *Service) PayOrder(ctx context.Context, buyerID string, orderNo string) (string, error) {
	order, err := s.repo.FindByOrderNo(ctx, orderNo)
	if err != nil {
		return "", err
	}
	if order.BuyerID != buyerID {
		return "", entity.ErrNotOrderOwner
	}
	now := time.Now().UTC()
	if !order.CanPay(now) {
		if order.IsExpired(now) {
			if _, err := s.closeExpired(ctx, orderNo, now); err != nil {
				return "", err
			}
			return "", fmt.Errorf("order closed: pay timeout")
		}
		return "", fmt.Errorf("order not payable in status %s", order.Status)
	}
	tradeNo, err := s.channel.Pay(ctx, orderNo, buyerID, order.AmountCents())
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateRecord(ctx, &entity.OrderRecord{
		OrderNo: orderNo, Type: entity.RecordTypePay, Operator: buyerID,
		Detail: fmt.Sprintf("渠道受理支付 trade_no=%s amount=%d", tradeNo, order.AmountCents()),
	}); err != nil {
		return "", fmt.Errorf("channel accepted trade %s but audit save failed: %w", tradeNo, err)
	}
	return tradeNo, nil
}
