package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"pagrin/internal/metrics"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

// ProviderPool manages multiple Ethereum RPC providers with automatic failover
// Implements weighted round-robin selection among healthy providers
type ProviderPool struct {
	providers []*Provider
	mu        sync.RWMutex
	current   int // Current provider index for round-robin
}

// NewProviderPool creates a new provider pool from a list of providers
// Providers should be sorted by weight (highest first) for optimal selection
func NewProviderPool(providers []*Provider) *ProviderPool {
	// Sort providers by weight (descending) for optimal selection
	sorted := make([]*Provider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Weight > sorted[j].Weight
	})

	return &ProviderPool{
		providers: sorted,
		current:   0,
	}
}

// GetHealthyProviders returns all currently healthy providers
// Used for selection and monitoring
func (p *ProviderPool) GetHealthyProviders() []*Provider {
	p.mu.RLock()
	defer p.mu.RUnlock()

	healthy := make([]*Provider, 0, len(p.providers))
	for _, provider := range p.providers {
		if provider.IsHealthy() {
			healthy = append(healthy, provider)
		}
	}
	return healthy
}

// SelectProvider returns the next healthy provider using weighted round-robin
// Falls back to unhealthy providers if all healthy ones are exhausted
func (p *ProviderPool) SelectProvider() (*Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	healthy := p.GetHealthyProviders()
	if len(healthy) > 0 {
		// Use round-robin among healthy providers
		selected := healthy[p.current%len(healthy)]
		p.current++
		return selected, nil
	}

	// All providers unhealthy - try any provider as last resort
	if len(p.providers) > 0 {
		return p.providers[0], fmt.Errorf("all providers unhealthy, using %s as fallback", p.providers[0].Name)
	}

	return nil, fmt.Errorf("no providers available")
}

// FilterLogs executes eth_getLogs with automatic failover across providers
// Tries each healthy provider in order until one succeeds
func (p *ProviderPool) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	// Calculate block range to determine which providers can handle this request
	blockRange := query.ToBlock.Uint64() - query.FromBlock.Uint64() + 1

	var lastErr error
	attemptedProviders := make(map[string]bool)

	// Try up to all providers (with retry logic)
	maxAttempts := len(p.providers) * 2 // Allow retry of each provider once
	for attempt := 0; attempt < maxAttempts; attempt++ {
		provider, err := p.SelectProvider()
		if err != nil {
			return nil, fmt.Errorf("no healthy providers available: %w", err)
		}

		// Check if provider supports this block range
		if blockRange > provider.MaxRange {
			// Skip this provider, try next
			attemptedProviders[provider.Name] = true
			lastErr = fmt.Errorf("provider %s max range (%d) exceeded by request (%d)", provider.Name, provider.MaxRange, blockRange)
			continue
		}

		// Check if we've already tried this provider
		if attemptedProviders[provider.Name] && attempt < len(p.providers) {
			// First pass - skip already attempted
			continue
		}

		// Create context with provider-specific timeout
		providerCtx, cancel := context.WithTimeout(ctx, provider.Timeout)
		defer cancel()

		start := time.Now()
		logs, err := provider.GetClient().FilterLogs(providerCtx, query)
		duration := time.Since(start)

		// Record metrics
		metrics.RPCRequestDuration.WithLabelValues(provider.Name, "FilterLogs").Observe(duration.Seconds())
		metrics.RPCRequestsTotal.WithLabelValues(provider.Name, "FilterLogs").Inc()

		if err == nil {
			// Success! Record which provider succeeded for observability
			provider.RecordSuccess()
			// Log provider used (if context has logger, otherwise metrics track it)
			return logs, nil
		}

		// Failure - record and try next provider
		provider.RecordFailure(err)
		lastErr = fmt.Errorf("provider %s failed: %w", provider.Name, err)
		attemptedProviders[provider.Name] = true

		// If context was cancelled, don't retry
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// BlockByNumber executes eth_getBlockByNumber with automatic failover
func (p *ProviderPool) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	var lastErr error
	attemptedProviders := make(map[string]bool)

	maxAttempts := len(p.providers) * 2
	for attempt := 0; attempt < maxAttempts; attempt++ {
		provider, err := p.SelectProvider()
		if err != nil {
			return nil, fmt.Errorf("no healthy providers available: %w", err)
		}

		if attemptedProviders[provider.Name] && attempt < len(p.providers) {
			continue
		}

		providerCtx, cancel := context.WithTimeout(ctx, provider.Timeout)
		defer cancel()

		start := time.Now()
		block, err := provider.GetClient().BlockByNumber(providerCtx, number)
		duration := time.Since(start)

		metrics.RPCRequestDuration.WithLabelValues(provider.Name, "BlockByNumber").Observe(duration.Seconds())
		metrics.RPCRequestsTotal.WithLabelValues(provider.Name, "BlockByNumber").Inc()

		if err == nil {
			provider.RecordSuccess()
			return block, nil
		}

		provider.RecordFailure(err)
		lastErr = fmt.Errorf("provider %s failed: %w", provider.Name, err)
		attemptedProviders[provider.Name] = true

		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// HeaderByNumber executes eth_getHeaderByNumber with automatic failover
func (p *ProviderPool) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	var lastErr error
	attemptedProviders := make(map[string]bool)

	maxAttempts := len(p.providers) * 2
	for attempt := 0; attempt < maxAttempts; attempt++ {
		provider, err := p.SelectProvider()
		if err != nil {
			return nil, fmt.Errorf("no healthy providers available: %w", err)
		}

		if attemptedProviders[provider.Name] && attempt < len(p.providers) {
			continue
		}

		providerCtx, cancel := context.WithTimeout(ctx, provider.Timeout)
		defer cancel()

		start := time.Now()
		header, err := provider.GetClient().HeaderByNumber(providerCtx, number)
		duration := time.Since(start)

		metrics.RPCRequestDuration.WithLabelValues(provider.Name, "HeaderByNumber").Observe(duration.Seconds())
		metrics.RPCRequestsTotal.WithLabelValues(provider.Name, "HeaderByNumber").Inc()

		if err == nil {
			provider.RecordSuccess()
			return header, nil
		}

		provider.RecordFailure(err)
		lastErr = fmt.Errorf("provider %s failed: %w", provider.Name, err)
		attemptedProviders[provider.Name] = true

		if ctx.Err() != nil {
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// Close closes all provider connections
func (p *ProviderPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, provider := range p.providers {
		provider.Close()
	}
}
