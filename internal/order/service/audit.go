package service

import (
	"context"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

func (s *Service) changeWithRecord(ctx context.Context, record *entity.OrderRecord, operation func(entity.Repository) (bool, error)) (bool, error) {
	if atomicRepo, ok := s.repo.(interface {
		WithAuditRecord(context.Context, *entity.OrderRecord, func(entity.Repository) (bool, error)) (bool, error)
	}); ok {
		return atomicRepo.WithAuditRecord(ctx, record, operation)
	}
	changed, err := operation(s.repo)
	if err != nil || !changed {
		return changed, err
	}
	if err := s.repo.CreateRecord(ctx, record); err != nil {
		return false, err
	}
	return true, nil
}
