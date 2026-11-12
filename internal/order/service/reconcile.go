package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// Completion receipts survive restarts. The stock service also performs a database
// compare-and-set, so replay after a lost response never adjusts quantity twice.
var stockSyncMu sync.Mutex

func (s *Service) syncStock(ctx context.Context, orderNo, action string) error {
	stockSyncMu.Lock()
	defer stockSyncMu.Unlock()
	records, err := s.repo.ListRecords(ctx, orderNo)
	if err != nil {
		return err
	}
	receipt := "stock-completed:" + action
	for _, r := range records {
		if r.Detail == receipt {
			return nil
		}
	}
	switch action {
	case "release":
		err = s.stock.Release(ctx, orderNo)
	case "confirm":
		err = s.stock.Confirm(ctx, orderNo)
	case "restore":
		// A lost payment-confirm response must be repaired before restoring stock.
		err = s.stock.Confirm(ctx, orderNo)
		if err == nil {
			err = s.stock.Restore(ctx, orderNo)
		}
	default:
		return fmt.Errorf("unknown stock action")
	}
	if err != nil {
		return err
	}
	return s.repo.CreateRecord(ctx, &entity.OrderRecord{OrderNo: orderNo, Type: entity.RecordTypeClose, Operator: "system", Detail: receipt})
}

// Terminal order states are durable retry intents. Scan after startup as well as
// periodically; no in-memory queue is required to recover a failed stock RPC.
func (s *Service) reconcileStock(ctx context.Context) error {
	for _, task := range []struct{ state, action string }{
		{entity.StatusCancelled, "release"}, {entity.StatusClosedTimeout, "release"},
		{entity.StatusPaid, "confirm"}, {entity.StatusShipped, "confirm"},
		{entity.StatusRefundRequested, "confirm"}, {entity.StatusRefundFailed, "confirm"},
		{entity.StatusRefunded, "restore"},
	} {
		orders, err := s.repo.ListByStatus(ctx, task.state)
		if err != nil {
			return err
		}
		for _, order := range orders {
			if err := s.syncStock(ctx, order.OrderNo, task.action); err != nil {
				return err
			}
		}
	}
	return nil
}
