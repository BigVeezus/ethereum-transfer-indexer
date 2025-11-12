package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"pagrin/internal/models"

	eth "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Fetcher struct {
	client *Client
	cache  *BlockHeaderCache // In-memory cache for block timestamps
}

// NewFetcher creates a new fetcher with optional block header cache
// Cache TTL defaults to 5 minutes (covers typical batch processing windows)
func NewFetcher(client *Client) *Fetcher {
	return &Fetcher{
		client: client,
		cache:  NewBlockHeaderCache(5 * time.Minute), // 5 minute TTL
	}
}

// FetchTransferLogs fetches Transfer event logs for a given block range
func (f *Fetcher) FetchTransferLogs(ctx context.Context, fromBlock, toBlock uint64) ([]*models.Transfer, error) {
	query := eth.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Topics: [][]common.Hash{
			{ERC20TransferEventSignature},
		},
	}

	// Use pool if available (supports failover), otherwise use single client
	var logs []types.Log
	var err error
	if pool := f.client.GetPool(); pool != nil {
		logs, err = pool.FilterLogs(ctx, query)
	} else {
		logs, err = f.client.GetClient().FilterLogs(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

	if len(logs) == 0 {
		return nil, nil
	}

	// Cache block timestamps to avoid fetching the same block multiple times
	blockTimestamps := make(map[uint64]time.Time)

	// Fetch unique block timestamps, checking cache first
	uniqueBlocks := make(map[uint64]bool)
	blocksToFetch := make([]uint64, 0) // Blocks not in cache

	for _, log := range logs {
		if !uniqueBlocks[log.BlockNumber] {
			uniqueBlocks[log.BlockNumber] = true
			// Check cache first
			if timestamp, found := f.cache.Get(log.BlockNumber); found {
				blockTimestamps[log.BlockNumber] = timestamp
			} else {
				blocksToFetch = append(blocksToFetch, log.BlockNumber)
			}
		}
	}

	// Only fetch blocks that aren't in cache
	if len(blocksToFetch) > 0 {
		// Fetch block headers in parallel (but limit concurrency)
		type blockResult struct {
			blockNum  uint64
			timestamp time.Time
			err       error
		}

		blockChan := make(chan blockResult, len(blocksToFetch))

		// Use a semaphore to limit concurrent requests (max 5 at a time)
		semaphore := make(chan struct{}, 5)

		for _, blockNum := range blocksToFetch {
			bn := blockNum // Capture loop variable
			go func() {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				blockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Use pool if available, otherwise single client
				var block *types.Block
				var err error
				if pool := f.client.GetPool(); pool != nil {
					block, err = pool.BlockByNumber(blockCtx, new(big.Int).SetUint64(bn))
				} else {
					block, err = f.client.GetClient().BlockByNumber(blockCtx, new(big.Int).SetUint64(bn))
				}

				if err != nil {
					blockChan <- blockResult{blockNum: bn, err: err}
					return
				}

				timestamp := time.Unix(int64(block.Time()), 0)
				// Store in cache for future use
				f.cache.Set(bn, timestamp)

				blockChan <- blockResult{
					blockNum:  bn,
					timestamp: timestamp,
				}
			}()
		}

		// Collect all block timestamps
		for i := 0; i < len(blocksToFetch); i++ {
			result := <-blockChan
			if result.err != nil {
				return nil, fmt.Errorf("failed to get block %d: %w", result.blockNum, result.err)
			}
			blockTimestamps[result.blockNum] = result.timestamp
		}
	}

	// Parse all logs using cached timestamps
	transfers := make([]*models.Transfer, 0, len(logs))
	for _, log := range logs {
		timestamp, ok := blockTimestamps[log.BlockNumber]
		if !ok {
			return nil, fmt.Errorf("missing timestamp for block %d", log.BlockNumber)
		}

		transfer, err := ParseTransferLog(log, timestamp)
		if err != nil {
			continue
		}

		transfers = append(transfers, transfer)
	}

	return transfers, nil
}

// GetBlockTimestamp retrieves the timestamp for a given block number
func (f *Fetcher) GetBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	var block *types.Block
	var err error

	// Use pool if available, otherwise single client
	if pool := f.client.GetPool(); pool != nil {
		block, err = pool.BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	} else {
		block, err = f.client.GetClient().BlockByNumber(ctx, new(big.Int).SetUint64(blockNumber))
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get block %d: %w", blockNumber, err)
	}
	return time.Unix(int64(block.Time()), 0), nil
}
