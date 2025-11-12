package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"pagrin/internal/ethereum"
	"pagrin/internal/metrics"
	"pagrin/internal/repository"
	"pagrin/pkg/logger"
)

// StreamPublisher interface for publishing events to stream
type StreamPublisher interface {
	Publish(transfer interface{})
}

type IngestionService struct {
	ethereumClient  *ethereum.Client
	fetcher         *ethereum.Fetcher
	repo            repository.Repository
	logger          *logger.Logger
	pollInterval    time.Duration
	startBlock      uint64
	blockBatchSize  uint64
	resetStartBlock bool
	stream          StreamPublisher // Optional stream for real-time events

	// Adaptive batch size state
	adaptiveBatch       bool
	batchMinSize        uint64
	batchMaxSize        uint64
	batchSuccessStreak  int
	batchFailureBackoff int
	currentBatchSize    uint64
	successCount        int
	failureCount        int
	mu                  sync.Mutex
}

func NewIngestionService(
	ethereumClient *ethereum.Client,
	fetcher *ethereum.Fetcher,
	repo repository.Repository,
	logger *logger.Logger,
	pollInterval time.Duration,
	startBlock uint64,
	blockBatchSize uint64,
	resetStartBlock bool,
	adaptiveBatch bool,
	batchMinSize uint64,
	batchMaxSize uint64,
	batchSuccessStreak int,
	batchFailureBackoff int,
	stream StreamPublisher,
) *IngestionService {
	// Initialize current batch size to configured starting size
	currentSize := blockBatchSize
	if currentSize == 0 {
		currentSize = 10
	}

	return &IngestionService{
		ethereumClient:      ethereumClient,
		fetcher:             fetcher,
		repo:                repo,
		logger:              logger,
		pollInterval:        pollInterval,
		startBlock:          startBlock,
		blockBatchSize:      blockBatchSize,
		resetStartBlock:     resetStartBlock,
		stream:              stream,
		adaptiveBatch:       adaptiveBatch,
		batchMinSize:        batchMinSize,
		batchMaxSize:        batchMaxSize,
		batchSuccessStreak:  batchSuccessStreak,
		batchFailureBackoff: batchFailureBackoff,
		currentBatchSize:    currentSize,
		successCount:        0,
		failureCount:        0,
	}
}

func (s *IngestionService) Start(ctx context.Context) error {
	var currentBlock uint64

	if s.resetStartBlock {
		s.logger.Info("RESET_START_BLOCK enabled, starting from configured START_BLOCK: %d", s.startBlock)
		currentBlock = s.startBlock
	} else {
		lastBlock, err := s.repo.GetLastProcessedBlock(ctx)
		if err != nil {
			return fmt.Errorf("failed to get last processed block: %w", err)
		}

		if lastBlock > 0 {
			currentBlock = lastBlock + 1
			s.logger.Info("Resuming from last processed block: %d (next: %d)", lastBlock, currentBlock)
		} else {
			currentBlock = s.startBlock
			s.logger.Info("No previous block found, starting from START_BLOCK: %d", currentBlock)
		}
	}

	s.logger.Info("Starting ingestion from block %d", currentBlock)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Ingestion stopped")
			return nil
		case <-ticker.C:
			nextBlock, err := s.processBlocks(ctx, currentBlock)
			if err != nil {
				metrics.IngestionErrorsTotal.WithLabelValues("processing").Inc()
				s.logger.Error("Failed to process blocks: %v", err)
				continue
			}
			currentBlock = nextBlock
		}
	}
}

func (s *IngestionService) processBlocks(ctx context.Context, fromBlock uint64) (uint64, error) {
	start := time.Now()
	defer func() {
		metrics.TransfersProcessingDuration.WithLabelValues("block_processing").Observe(time.Since(start).Seconds())
	}()

	// Add timeout to prevent hanging
	processCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	latestBlock, err := s.ethereumClient.GetLatestBlockNumber(processCtx)
	if err != nil {
		return fromBlock, fmt.Errorf("failed to get latest block: %w", err)
	}

	if fromBlock > latestBlock {
		return fromBlock, nil
	}

	// Get current batch size (may be adjusted by adaptive logic)
	s.mu.Lock()
	batchSize := s.currentBatchSize
	if batchSize == 0 {
		batchSize = s.blockBatchSize
		if batchSize == 0 {
			batchSize = 10
		}
	}
	s.mu.Unlock()

	toBlock := fromBlock + batchSize - 1
	if toBlock > latestBlock {
		toBlock = latestBlock
	}

	s.logger.Debug("Fetching transfers from blocks %d-%d (batch size: %d)", fromBlock, toBlock, batchSize)
	transfers, err := s.fetcher.FetchTransferLogs(processCtx, fromBlock, toBlock)
	if err != nil {
		// Record failure and adjust batch size if adaptive mode is enabled
		if s.adaptiveBatch {
			s.adjustBatchSizeOnFailure()
		}
		return fromBlock, fmt.Errorf("failed to fetch transfer logs: %w", err)
	}

	if len(transfers) > 0 {
		if err := s.repo.InsertTransfers(ctx, transfers); err != nil {
			// Insert failure - adjust batch size if adaptive
			if s.adaptiveBatch {
				s.adjustBatchSizeOnFailure()
			}
			return fromBlock, fmt.Errorf("failed to insert transfers: %w", err)
		}
		metrics.TransfersProcessedTotal.WithLabelValues("success").Add(float64(len(transfers)))
		// Log with structured fields for better observability
		s.logger.WithFields("info", "Processed transfers", map[string]interface{}{
			"transfers":  len(transfers),
			"from_block": fromBlock,
			"to_block":   toBlock,
			"batch_size": batchSize,
			"elapsed_ms": time.Since(start).Milliseconds(),
		})
		s.logger.Info("Processed %d transfers from blocks %d-%d (batch: %d)", len(transfers), fromBlock, toBlock, batchSize)

		// Publish transfers to stream if enabled
		// Check if stream interface is properly initialized (not nil)
		if s.stream != nil {
			for _, transfer := range transfers {
				if transfer != nil {
					s.stream.Publish(transfer)
				}
			}
		}
	} else {
		s.logger.Debug("No transfers found in blocks %d-%d", fromBlock, toBlock)
	}

	if err := s.repo.SetLastProcessedBlock(ctx, toBlock); err != nil {
		return fromBlock, fmt.Errorf("failed to set last processed block: %w", err)
	}

	metrics.BlocksProcessedTotal.Add(float64(toBlock - fromBlock + 1))

	// Record success and adjust batch size if adaptive mode is enabled
	if s.adaptiveBatch {
		s.adjustBatchSizeOnSuccess()
	}

	// Check if processing took longer than poll interval (back-pressure indicator)
	processingTime := time.Since(start)
	if processingTime > s.pollInterval {
		s.logger.Warn("Batch processing took %v (longer than poll interval %v) - consider reducing batch size", processingTime, s.pollInterval)
	}

	return toBlock + 1, nil
}

// adjustBatchSizeOnSuccess increases batch size after successful streaks
// Implements exponential back-on strategy: double size after N successes
func (s *IngestionService) adjustBatchSizeOnSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.successCount++
	s.failureCount = 0

	// Increase batch size after success streak
	if s.successCount >= s.batchSuccessStreak {
		newSize := s.currentBatchSize * 2
		if newSize > s.batchMaxSize {
			newSize = s.batchMaxSize
		}
		if newSize != s.currentBatchSize {
			s.logger.Info("Increasing batch size from %d to %d (success streak: %d)", s.currentBatchSize, newSize, s.successCount)
			s.currentBatchSize = newSize
			s.successCount = 0 // Reset counter after adjustment
		}
	}
}

// adjustBatchSizeOnFailure decreases batch size on failures
// Implements exponential backoff: halve size on failure
func (s *IngestionService) adjustBatchSizeOnFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failureCount++
	s.successCount = 0

	// Decrease batch size on failure
	newSize := s.currentBatchSize / uint64(s.batchFailureBackoff)
	if newSize < s.batchMinSize {
		newSize = s.batchMinSize
	}
	if newSize != s.currentBatchSize {
		s.logger.Warn("Decreasing batch size from %d to %d (failure count: %d)", s.currentBatchSize, newSize, s.failureCount)
		s.currentBatchSize = newSize
		s.failureCount = 0 // Reset counter after adjustment
	}
}
