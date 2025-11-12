package ethereum

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Client wraps either a single ethclient or a ProviderPool
// Provides backward compatibility while supporting multi-provider failover
type Client struct {
	client  *ethclient.Client // Single client (legacy mode)
	pool    *ProviderPool     // Provider pool (new mode)
	usePool bool              // Whether to use pool or single client
}

// NewClient creates a client from a single RPC URL (legacy mode)
// For production, use NewClientFromPool instead
func NewClient(rpcURL string) (*Client, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	return &Client{
		client:  client,
		usePool: false,
	}, nil
}

// NewClientFromPool creates a client using a provider pool
// This is the recommended approach for production with failover support
func NewClientFromPool(pool *ProviderPool) *Client {
	return &Client{
		pool:    pool,
		usePool: true,
	}
}

// GetLatestBlockNumber retrieves the latest block number
// Uses pool if available, otherwise falls back to single client
func (c *Client) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	if c.usePool && c.pool != nil {
		header, err := c.pool.HeaderByNumber(ctx, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to get latest block: %w", err)
		}
		return header.Number.Uint64(), nil
	}

	if c.client == nil {
		return 0, fmt.Errorf("no client or pool available")
	}

	header, err := c.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}
	return header.Number.Uint64(), nil
}

// GetBlockNumber retrieves the latest block number as *big.Int
func (c *Client) GetBlockNumber(ctx context.Context) (*big.Int, error) {
	if c.usePool && c.pool != nil {
		header, err := c.pool.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		return header.Number, nil
	}

	if c.client == nil {
		return nil, fmt.Errorf("no client or pool available")
	}

	header, err := c.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}
	return header.Number, nil
}

// GetClient returns the underlying ethclient (legacy support)
// Returns nil if using pool mode
func (c *Client) GetClient() *ethclient.Client {
	return c.client
}

// GetPool returns the provider pool if available
func (c *Client) GetPool() *ProviderPool {
	return c.pool
}

// Close closes all connections
func (c *Client) Close() {
	if c.client != nil {
		c.client.Close()
	}
	if c.pool != nil {
		c.pool.Close()
	}
}
