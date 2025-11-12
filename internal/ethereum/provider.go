package ethereum

import (
	"fmt"
	"sync"
	"time"

	"pagrin/internal/metrics"

	"github.com/ethereum/go-ethereum/ethclient"
)

// ProviderState represents the health state of an RPC provider
type ProviderState int

const (
	StateHealthy ProviderState = iota
	StateUnhealthy
	StateHalfOpen // Testing if provider recovered
)

// Provider represents a single Ethereum RPC endpoint with health tracking
type Provider struct {
	Name     string
	URL      string
	Weight   int
	MaxRange uint64 // Maximum block range for eth_getLogs
	Timeout  time.Duration

	client *ethclient.Client

	// Circuit breaker state
	mu              sync.RWMutex
	state           ProviderState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	lastSuccessTime time.Time

	// Circuit breaker config
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	halfOpenMaxCalls int
	halfOpenCalls    int
}

// NewProvider creates a new provider instance
func NewProvider(name, url string, weight int, maxRange uint64, timeout time.Duration, cbConfig CircuitBreakerConfig) (*Provider, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to provider %s: %w", name, err)
	}

	return &Provider{
		Name:             name,
		URL:              url,
		Weight:           weight,
		MaxRange:         maxRange,
		Timeout:          timeout,
		client:           client,
		state:            StateHealthy,
		failureThreshold: cbConfig.FailureThreshold,
		successThreshold: cbConfig.SuccessThreshold,
		timeout:          cbConfig.Timeout,
		halfOpenMaxCalls: cbConfig.HalfOpenMaxCalls,
	}, nil
}

// IsHealthy returns true if provider is in healthy state
func (p *Provider) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.state == StateHealthy {
		return true
	}

	// Check if timeout has passed for unhealthy/half-open states
	if p.state == StateUnhealthy {
		if time.Since(p.lastFailureTime) > p.timeout {
			// Move to half-open state
			p.mu.RUnlock()
			p.mu.Lock()
			if p.state == StateUnhealthy && time.Since(p.lastFailureTime) > p.timeout {
				p.state = StateHalfOpen
				p.halfOpenCalls = 0
			}
			p.mu.Unlock()
			p.mu.RLock()
			return p.state == StateHalfOpen
		}
		return false
	}

	// Half-open state
	return p.state == StateHalfOpen
}

// RecordSuccess marks a successful call and updates circuit breaker state
func (p *Provider) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastSuccessTime = time.Now()
	p.successCount++
	p.failureCount = 0

	// Update metrics
	metrics.RPCRequestsTotal.WithLabelValues(p.Name, "success").Inc()

	// State transitions
	if p.state == StateHalfOpen {
		p.halfOpenCalls++
		if p.successCount >= p.successThreshold {
			p.state = StateHealthy
			p.halfOpenCalls = 0
			p.successCount = 0
		}
	} else if p.state == StateUnhealthy {
		// Shouldn't happen, but handle gracefully
		p.state = StateHalfOpen
	}
}

// RecordFailure marks a failed call and updates circuit breaker state
func (p *Provider) RecordFailure(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastFailureTime = time.Now()
	p.failureCount++
	p.successCount = 0

	// Extract error code if available
	errorCode := "unknown"
	if err != nil {
		errorCode = err.Error()
	}

	// Update metrics
	metrics.RPCErrorsTotal.WithLabelValues(p.Name, errorCode).Inc()

	// State transitions
	if p.state == StateHalfOpen {
		// Any failure in half-open immediately goes to unhealthy
		p.state = StateUnhealthy
		p.halfOpenCalls = 0
	} else if p.failureCount >= p.failureThreshold {
		p.state = StateUnhealthy
	}
}

// GetClient returns the ethclient for this provider
func (p *Provider) GetClient() *ethclient.Client {
	return p.client
}

// Close closes the provider's client connection
func (p *Provider) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

// CircuitBreakerConfig holds circuit breaker parameters
type CircuitBreakerConfig struct {
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	HalfOpenMaxCalls int
}

// DefaultCircuitBreakerConfig returns sensible defaults for circuit breaker
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		HalfOpenMaxCalls: 3,
	}
}
