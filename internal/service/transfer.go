package service

import (
	"context"
	"fmt"

	"pagrin/internal/models"
	"pagrin/internal/repository"
	"pagrin/pkg/logger"
)

type TransferService struct {
	repo   repository.Repository
	logger *logger.Logger
}

func NewTransferService(repo repository.Repository, logger *logger.Logger) *TransferService {
	return &TransferService{
		repo:   repo,
		logger: logger,
	}
}

func (s *TransferService) ProcessTransfers(ctx context.Context, transfers []*models.Transfer) error {
	if len(transfers) == 0 {
		return nil
	}

	if err := s.repo.InsertTransfers(ctx, transfers); err != nil {
		return fmt.Errorf("failed to insert transfers: %w", err)
	}

	s.logger.Info("Processed %d transfers", len(transfers))
	return nil
}

func (s *TransferService) QueryTransfers(ctx context.Context, params models.TransferQueryParams) ([]*models.Transfer, int64, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	transfers, total, err := s.repo.QueryTransfers(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query transfers: %w", err)
	}

	return transfers, total, nil
}

func (s *TransferService) GetAggregates(ctx context.Context, params models.TransferQueryParams) (*models.AggregateResponse, error) {
	aggregates, err := s.repo.GetAggregates(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregates: %w", err)
	}

	return aggregates, nil
}
