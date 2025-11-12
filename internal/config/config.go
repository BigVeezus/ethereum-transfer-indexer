package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig
	Ethereum  EthereumConfig
	MongoDB   MongoDBConfig
	Redis     RedisConfig
	Ingestion IngestionConfig
	Logging   LoggingConfig
	Streaming StreamingConfig
}

type ServerConfig struct {
	Port string
}

type EthereumConfig struct {
	RPCURL    string // Single RPC URL (legacy mode)
	RPCConfig string // Path to provider YAML config (preferred)
}

type MongoDBConfig struct {
	URI      string
	Database string
}

type RedisConfig struct {
	URI     string
	Enabled bool
}

type LoggingConfig struct {
	Level      string
	ToFile     bool
	FilePath   string
	Format     string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
}

type IngestionConfig struct {
	StartBlock          uint64
	PollInterval        time.Duration
	BlockBatchSize      uint64
	ResetStartBlock     bool
	AdaptiveBatch       bool
	BatchMinSize        uint64
	BatchMaxSize        uint64
	BatchSuccessStreak  int
	BatchFailureBackoff int
}

type StreamingConfig struct {
	Enabled    bool
	Type       string // "ws" or "sse"
	Route      string
	BufferSize int
	Port       string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// .env file is optional
	}

	cfg := &Config{}

	cfg.Server.Port = getEnv("SERVER_PORT", "8080")
	cfg.Ethereum.RPCURL = getEnv("ETH_RPC_URL", "")
	cfg.Ethereum.RPCConfig = getEnv("RPC_CONFIG", "")
	cfg.MongoDB.URI = getEnv("MONGODB_URI", "mongodb://localhost:27017")
	cfg.MongoDB.Database = getEnv("MONGODB_DB", "ethereum")

	cfg.Redis.URI = getEnv("REDIS_URI", "redis://localhost:6379")
	redisEnabled := getEnv("USE_REDIS", "true")
	cfg.Redis.Enabled = redisEnabled == "true" || redisEnabled == "1"

	// Logging configuration
	cfg.Logging.Level = getEnv("LOG_LEVEL", "info")
	logToFile := getEnv("LOG_TO_FILE", "false")
	cfg.Logging.ToFile = logToFile == "true" || logToFile == "1"
	cfg.Logging.FilePath = getEnv("LOG_FILE_PATH", "logs/app.log")
	cfg.Logging.Format = getEnv("LOG_FORMAT", "text") // "text" or "json"

	maxSizeMB, err := strconv.Atoi(getEnv("LOG_MAX_SIZE_MB", "100"))
	if err == nil {
		cfg.Logging.MaxSizeMB = maxSizeMB
	} else {
		cfg.Logging.MaxSizeMB = 100
	}

	maxBackups, err := strconv.Atoi(getEnv("LOG_MAX_BACKUPS", "7"))
	if err == nil {
		cfg.Logging.MaxBackups = maxBackups
	} else {
		cfg.Logging.MaxBackups = 7
	}

	maxAgeDays, err := strconv.Atoi(getEnv("LOG_MAX_AGE_DAYS", "30"))
	if err == nil {
		cfg.Logging.MaxAgeDays = maxAgeDays
	} else {
		cfg.Logging.MaxAgeDays = 30
	}

	// Streaming configuration
	streamEnabled := getEnv("ENABLE_STREAM", "false")
	cfg.Streaming.Enabled = streamEnabled == "true" || streamEnabled == "1"
	cfg.Streaming.Type = getEnv("STREAM_TYPE", "ws") // "ws" or "sse"
	cfg.Streaming.Route = getEnv("STREAM_ROUTE", "/ws")
	bufferSize, err := strconv.Atoi(getEnv("STREAM_BUFFER", "1024"))
	if err == nil {
		cfg.Streaming.BufferSize = bufferSize
	} else {
		cfg.Streaming.BufferSize = 1024
	}
	cfg.Streaming.Port = getEnv("STREAM_PORT", "8090")

	startBlock, err := strconv.ParseUint(getEnv("START_BLOCK", "0"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid START_BLOCK: %w", err)
	}
	cfg.Ingestion.StartBlock = startBlock

	pollInterval, err := strconv.Atoi(getEnv("POLL_INTERVAL", "12"))
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
	}
	cfg.Ingestion.PollInterval = time.Duration(pollInterval) * time.Second

	blockBatchSize, err := strconv.ParseUint(getEnv("BLOCK_BATCH_SIZE", "10"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid BLOCK_BATCH_SIZE: %w", err)
	}
	if blockBatchSize == 0 {
		blockBatchSize = 10
	}
	if blockBatchSize > 100 {
		blockBatchSize = 100
	}
	cfg.Ingestion.BlockBatchSize = blockBatchSize

	resetStartBlock := getEnv("RESET_START_BLOCK", "false")
	cfg.Ingestion.ResetStartBlock = resetStartBlock == "true" || resetStartBlock == "1"

	// Adaptive batch size configuration
	adaptiveBatch := getEnv("ADAPTIVE_BATCH", "true")
	cfg.Ingestion.AdaptiveBatch = adaptiveBatch == "true" || adaptiveBatch == "1"

	batchMinSize, err := strconv.ParseUint(getEnv("BATCH_MIN_SIZE", "1"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_MIN_SIZE: %w", err)
	}
	cfg.Ingestion.BatchMinSize = batchMinSize

	batchMaxSize, err := strconv.ParseUint(getEnv("BATCH_MAX_SIZE", "100"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_MAX_SIZE: %w", err)
	}
	cfg.Ingestion.BatchMaxSize = batchMaxSize

	// If adaptive batch is disabled, use fixed batch size
	if !cfg.Ingestion.AdaptiveBatch {
		cfg.Ingestion.BatchMinSize = blockBatchSize
		cfg.Ingestion.BatchMaxSize = blockBatchSize
	}

	batchSuccessStreak, err := strconv.Atoi(getEnv("BATCH_SUCCESS_STREAK", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_SUCCESS_STREAK: %w", err)
	}
	cfg.Ingestion.BatchSuccessStreak = batchSuccessStreak

	batchFailureBackoff, err := strconv.Atoi(getEnv("BATCH_FAILURE_BACKOFF", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_FAILURE_BACKOFF: %w", err)
	}
	cfg.Ingestion.BatchFailureBackoff = batchFailureBackoff

	// Either RPC_CONFIG (YAML) or ETH_RPC_URL (single provider) must be provided
	if cfg.Ethereum.RPCConfig == "" && cfg.Ethereum.RPCURL == "" {
		return nil, fmt.Errorf("either RPC_CONFIG or ETH_RPC_URL must be provided")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
