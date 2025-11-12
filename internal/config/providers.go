package config

import (
	"fmt"
	"os"
	"time"

	"pagrin/internal/ethereum"

	"gopkg.in/yaml.v3"
)

// ProviderConfig represents a single RPC provider configuration
type ProviderConfig struct {
	Name     string        `yaml:"name"`
	URL      string        `yaml:"url"`
	Weight   int           `yaml:"weight"`
	MaxRange uint64        `yaml:"maxRange"`
	Timeout  time.Duration `yaml:"timeout"`
}

// ProvidersConfig holds the complete provider configuration
type ProvidersConfig struct {
	Providers      []ProviderConfig   `yaml:"providers"`
	CircuitBreaker CircuitBreakerYAML `yaml:"circuit_breaker"`
}

// CircuitBreakerYAML holds circuit breaker configuration from YAML
type CircuitBreakerYAML struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
	HalfOpenMaxCalls int           `yaml:"half_open_max_calls"`
}

// LoadProvidersFromYAML loads provider configuration from a YAML file
// Falls back to single provider from env if file doesn't exist
func LoadProvidersFromYAML(filePath string, fallbackURL string) ([]*ethereum.Provider, error) {
	// Try to load from YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist or can't be read - use fallback
		if fallbackURL == "" {
			return nil, fmt.Errorf("no provider config file found and no fallback URL provided")
		}

		// Create single provider from env fallback
		cbConfig := ethereum.DefaultCircuitBreakerConfig()
		provider, err := ethereum.NewProvider(
			"default",
			fallbackURL,
			10,
			10, // Default max range for free tier
			30*time.Second,
			cbConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fallback provider: %w", err)
		}

		return []*ethereum.Provider{provider}, nil
	}

	var config ProvidersConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse provider config: %w", err)
	}

	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("no providers configured in YAML file")
	}

	// Convert YAML config to circuit breaker config
	cbConfig := ethereum.CircuitBreakerConfig{
		FailureThreshold: config.CircuitBreaker.FailureThreshold,
		SuccessThreshold: config.CircuitBreaker.SuccessThreshold,
		Timeout:          config.CircuitBreaker.Timeout,
		HalfOpenMaxCalls: config.CircuitBreaker.HalfOpenMaxCalls,
	}

	// Use defaults if not specified
	if cbConfig.FailureThreshold == 0 {
		cbConfig = ethereum.DefaultCircuitBreakerConfig()
	}

	// Create providers
	providers := make([]*ethereum.Provider, 0, len(config.Providers))
	for _, pConfig := range config.Providers {
		if pConfig.URL == "" {
			continue // Skip invalid entries
		}

		// Set defaults
		if pConfig.Weight == 0 {
			pConfig.Weight = 1
		}
		if pConfig.MaxRange == 0 {
			pConfig.MaxRange = 10 // Safe default for free tier
		}
		if pConfig.Timeout == 0 {
			pConfig.Timeout = 30 * time.Second
		}

		provider, err := ethereum.NewProvider(
			pConfig.Name,
			pConfig.URL,
			pConfig.Weight,
			pConfig.MaxRange,
			pConfig.Timeout,
			cbConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", pConfig.Name, err)
		}

		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid providers created from config")
	}

	return providers, nil
}
