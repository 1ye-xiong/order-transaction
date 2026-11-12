package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// ShipOrder 仓库发货：仅合法待发货订单可发货一次；退款申请期间暂停发货。
func (s *Service) ShipOrder(ctx context.Context, cmd dto.ShipCommand) (*dto.OrderDTO, error) {
	if cmd.Carrier == "" || cmd.TrackingNo == "" {
		return nil, entity.ErrTrackingNoRequired
	}
	order, err := s.repo.FindByOrderNo(ctx, cmd.OrderNo)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	changed, err := s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: cmd.OrderNo, Type: entity.RecordTypeShip, Operator: cmd.Operator, Detail: fmt.Sprintf("发货登记 carrier=%s tracking_no=%s", cmd.Carrier, cmd.TrackingNo)}, func(r entity.Repository) (bool, error) {
		return r.Transition(ctx, cmd.OrderNo, entity.StatusPaid, entity.StatusShipped, func(o *entity.Order) {
			o.Carrier = cmd.Carrier
			o.TrackingNo = cmd.TrackingNo
			shippedAt := now
			o.ShippedAt = &shippedAt
		}, now)
	})
	if err != nil {
		return nil, err
	}
	if !changed {
		order, err = s.repo.FindByOrderNo(ctx, cmd.OrderNo)
		if err != nil {
			return nil, err
		}
		return nil, shipBlockedError(order.Status)
	}
	order, err = s.repo.FindByOrderNo(ctx, cmd.OrderNo)
	if err != nil {
		return nil, err
	}
	return dto.FromOrder(order), nil
}

// ListShippableOrders 仓库查询可发货订单。
func (s *Service) ListShippableOrders(ctx context.Context) ([]*dto.OrderDTO, error) {
	orders, err := s.repo.ListByStatus(ctx, entity.StatusPaid)
	if err != nil {
		return nil, err
	}
	return dto.FromOrders(orders), nil
}

// ListOrderRecords 运营按订单核查收款、退款、发货、关闭与异常处理流水。
func (s *Service) ListOrderRecords(ctx context.Context, orderNo string) ([]*entity.OrderRecord, error) {
	return s.repo.ListRecords(ctx, orderNo)
}

// OpsSummary 运营交易汇总：已收款、已退款、各状态订单数、异常支付数。
func (s *Service) OpsSummary(ctx context.Context) (int64, int64, map[string]int64, int64, error) {
	return s.repo.Summary(ctx)
}

// CloseExpiredOrders 超时关闭扫描（后台定时任务调用）：服务端时间判断，幂等。
func (s *Service) CloseExpiredOrders(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	expired, err := s.repo.ListExpiredPending(ctx, now)
	if err != nil {
		return 0, err
	}
	closed := 0
	for _, order := range expired {
		ok, err := s.closeExpired(ctx, order.OrderNo, now)
		if err != nil {
			return closed, err
		}
		if ok {
			closed++
		}
	}
	if err := s.reconcileStock(ctx); err != nil {
		return closed, err
	}
	return closed, nil
}

func (s *Service) closeExpired(ctx context.Context, orderNo string, now time.Time) (bool, error) {
	changed, err := s.changeWithRecord(ctx, &entity.OrderRecord{OrderNo: orderNo, Type: entity.RecordTypeClose, Operator: "system", Detail: "待付款超时关闭"}, func(r entity.Repository) (bool, error) { return r.CloseIfExpired(ctx, orderNo, now) })
	if err != nil || !changed {
		return false, err
	}
	// 超时关闭释放占用；重复关闭不多释放（条件更新天然幂等）
	if err := s.syncStock(ctx, orderNo, "release"); err != nil {
		return true, err
	}
	return true, nil
}

func normalizePagination(page int32, pageSize int32) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return int((page - 1) * pageSize), int(pageSize)
}

func newOrderNo() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ORD" + strconv.FormatInt(time.Now().UnixMilli(), 10) + hex.EncodeToString(buf), nil
}

func shipBlockedError(status string) error {
	switch status {
	case entity.StatusRefundRequested:
		return fmt.Errorf("refund requested: shipping paused")
	case entity.StatusShipped:
		return fmt.Errorf("order already shipped")
	case entity.StatusRefunded:
		return fmt.Errorf("order refunded: shipping rejected")
	case entity.StatusRefundFailed:
		return fmt.Errorf("refund failed pending retry: shipping paused")
	default:
		return fmt.Errorf("order not shippable in status %s", status)
	}
}
