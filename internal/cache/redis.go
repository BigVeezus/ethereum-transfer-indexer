package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"pagrin/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Cache provides fast look-aside caching for ingestion state
// Uses Redis for high-performance reads/writes of frequently accessed data
type Cache interface {
	GetLastProcessedBlock(ctx context.Context) (uint64, error)
	SetLastProcessedBlock(ctx context.Context, blockNumber uint64) error
	IsTxProcessed(ctx context.Context, txHash string) (bool, error)
	MarkTxProcessed(ctx context.Context, txHash string) error
	Close() error
}

// RedisCache implements Cache interface using Redis
// Provides sub-millisecond access to last processed block and duplicate detection
type RedisCache struct {
	client  *redis.Client
	logger  *logger.Logger
	enabled bool
}

// NewRedisCache creates a new Redis cache instance
// If Redis is unavailable, returns a no-op cache that gracefully degrades
func NewRedisCache(uri string, enabled bool, log *logger.Logger) (*RedisCache, error) {
	if !enabled {
		log.Info("Redis cache disabled, using MongoDB-only mode")
		return &RedisCache{enabled: false, logger: log}, nil
	}

	opts, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URI: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Info("Redis cache connected successfully")

	return &RedisCache{
		client:  client,
		logger:  log,
		enabled: true,
	}, nil
}

const (
	// Key prefixes for Redis keys
	keyLastBlock = "ethereum:last_block"
	keyTxPrefix  = "ethereum:tx:"

	// TTL for transaction hash cache (24 hours)
	// Prevents reprocessing transactions in case of chain reorganizations
	txCacheTTL = 24 * time.Hour
)

// GetLastProcessedBlock retrieves the last processed block number from Redis
// Returns 0 if key doesn't exist (first run)
func (r *RedisCache) GetLastProcessedBlock(ctx context.Context) (uint64, error) {
	if !r.enabled {
		return 0, ErrCacheDisabled
	}

	val, err := r.client.Get(ctx, keyLastBlock).Result()
	if err == redis.Nil {
		// Key doesn't exist - first run or cache was cleared
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get last block from Redis: %w", err)
	}

	blockNum, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid block number in cache: %w", err)
	}

	return blockNum, nil
}

// SetLastProcessedBlock stores the last processed block number in Redis
// Uses SET with no expiration (persistent until manually cleared)
func (r *RedisCache) SetLastProcessedBlock(ctx context.Context, blockNumber uint64) error {
	if !r.enabled {
		return ErrCacheDisabled
	}

	val := strconv.FormatUint(blockNumber, 10)
	if err := r.client.Set(ctx, keyLastBlock, val, 0).Err(); err != nil {
		return fmt.Errorf("failed to set last block in Redis: %w", err)
	}

	return nil
}

// IsTxProcessed checks if a transaction hash has been processed
// Uses Redis SET with TTL for efficient duplicate detection
func (r *RedisCache) IsTxProcessed(ctx context.Context, txHash string) (bool, error) {
	if !r.enabled {
		return false, ErrCacheDisabled
	}

	key := keyTxPrefix + txHash
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check tx in Redis: %w", err)
	}

	return exists > 0, nil
}

// MarkTxProcessed marks a transaction hash as processed
// Sets key with TTL to automatically expire old entries
func (r *RedisCache) MarkTxProcessed(ctx context.Context, txHash string) error {
	if !r.enabled {
		return ErrCacheDisabled
	}

	key := keyTxPrefix + txHash
	if err := r.client.Set(ctx, key, "1", txCacheTTL).Err(); err != nil {
		return fmt.Errorf("failed to mark tx in Redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection gracefully
func (r *RedisCache) Close() error {
	if !r.enabled || r.client == nil {
		return nil
	}

	return r.client.Close()
}

// ErrCacheDisabled is returned when cache operations are attempted but Redis is disabled
var ErrCacheDisabled = fmt.Errorf("cache disabled")
