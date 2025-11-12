# Ethereum ERC-20 Transfer Event Ingestion Service

## Architecture Overview

### High-Level Design

```
┌─────────────────────────────────────────────────────────────┐
│                    Ethereum RPC Providers                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
│  │ Alchemy  │  │  Infura   │  │  Backup  │               │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘               │
│       │             │             │                       │
│       └─────────────┴─────────────┘                       │
│                    │                                       │
│                    ▼                                       │
│         ┌──────────────────────┐                          │
│         │   Provider Pool      │                          │
│         │  (Circuit Breaker)   │                          │
│         │  (Failover Logic)     │                          │
│         └──────────┬───────────┘                          │
└────────────────────┼──────────────────────────────────────┘
                    │
                    ▼
┌───────────────────────────────────────────────────────────┐
│              Event Ingestion Service                       │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Fetcher (with Block Header Cache)                  │  │
│  │  • Parallel block fetching                         │  │
│  │  • In-memory timestamp cache (5min TTL)           │  │
│  └──────────────────┬─────────────────────────────────┘  │
│                     │                                      │
│                     ▼                                      │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Parser (Normalizer)                                │  │
│  │  • Decimal128 value conversion                      │  │
│  │  • Event signature tagging                          │  │
│  └──────────────────┬─────────────────────────────────┘  │
│                     │                                      │
│                     ▼                                      │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Repository (BulkWrite)                            │  │
│  │  • Optimized batch inserts                         │  │
│  │  • Idempotency via composite index                 │  │
│  └──────────────────┬─────────────────────────────────┘  │
└─────────────────────┼──────────────────────────────────────┘
                      │
        ┌─────────────┴─────────────┐
        │                           │
        ▼                           ▼
┌──────────────┐          ┌─────────────────┐
│   MongoDB    │          │      Redis      │
│  (Primary)   │          │   (Cache)       │
│              │          │                 │
│ • transfers  │          │ • last_block    │
│ • processed_ │          │ • tx_hash (TTL) │
│   blocks     │          │                 │
└──────────────┘          └─────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────┐
│              Service Layer                                 │
│  • Adaptive Batch Sizing (exponential back-on/backoff)    │
│  • Stream Publisher (WebSocket/SSE)                       │
│  • Metrics Collection                                     │
└──────────────────┬─────────────────────────────────────────┘
                   │
                   ▼
┌───────────────────────────────────────────────────────────┐
│              HTTP API (Gin)                                │
│  • RESTful endpoints                                      │
│  • WebSocket/SSE streaming                                │
│  • Prometheus metrics                                     │
└──────────────────┬─────────────────────────────────────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
        ▼                     ▼
┌──────────────┐    ┌─────────────────┐
│  Prometheus  │    │    Grafana      │
│  (Metrics)   │    │ (Visualization) │
└──────────────┘    └─────────────────┘
```

## Project Structure

```
pagrin/
├── cmd/
│   └── server/
│       └── main.go                    # Application entry point
├── internal/
│   ├── config/
│   │   ├── config.go                 # Configuration management
│   │   └── providers.go              # Provider YAML config loader
│   ├── ethereum/
│   │   ├── client.go                 # Ethereum client wrapper
│   │   ├── fetcher.go                # Log fetching with caching
│   │   ├── parser.go                 # Event parsing/normalization
│   │   ├── provider.go               # Provider with circuit breaker
│   │   ├── pool.go                   # Provider pool with failover
│   │   └── cache.go                  # Block header cache
│   ├── models/
│   │   └── transfer.go               # Transfer event model (Decimal128)
│   ├── repository/
│   │   └── mongo.go                  # MongoDB repository (BulkWrite)
│   ├── service/
│   │   ├── transfer.go               # Business logic layer
│   │   └── ingestion.go              # Ingestion with adaptive batching
│   ├── handler/
│   │   ├── transfer.go               # HTTP handlers
│   │   └── stream.go                 # WebSocket/SSE handlers
│   ├── metrics/
│   │   └── metrics.go                # Prometheus metrics
│   ├── cache/
│   │   └── redis.go                  # Redis cache layer
│   └── stream/
│       └── stream.go                  # Event streaming service
├── pkg/
│   └── logger/
│       └── logger.go                  # Structured logging with rotation
├── config/
│   └── providers.example.yaml        # Provider configuration template
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── .env.example
└── README.md
```

## Component Responsibilities

### 1. Config Package

- Load configuration from environment variables
- Load provider configuration from YAML (with fallback to env)
- Validate required settings
- Provide typed configuration structs

### 2. Ethereum Package

- **Client**: Wraps go-ethereum client or provider pool
- **Provider**: Individual RPC provider with circuit breaker
- **ProviderPool**: Manages multiple providers with failover
- **Fetcher**: Polls Ethereum node for new Transfer events with caching
- **Parser**: Normalizes raw logs into structured Transfer events
- **Cache**: In-memory block header cache (5-minute TTL)

### 3. Models Package

- Defines Transfer event structure with Decimal128 value
- Event signature field for future extensibility
- MongoDB document models
- API request/response DTOs

### 4. Repository Package

- Interface-based design for testability
- MongoDB operations with BulkWrite optimization (~30% faster)
- Ordered queries using `bson.D` for proper sort ordering
- Redis integration for fast lookups (look-aside cache)
- Index management with composite unique index
- Connection pooling

### 5. Service Package

- **TransferService**: Business logic for transfer operations
- **IngestionService**: Orchestrates continuous event ingestion
  - Adaptive batch sizing (exponential back-on/backoff)
  - Stream integration for real-time events
  - Metrics integration

### 6. Handler Package

- HTTP route handlers
- WebSocket/SSE streaming handlers
- Request validation
- Response formatting
- Error handling

### 7. Metrics Package

- Prometheus metric definitions
- Provider-level metrics (requests, errors, duration)
- Ingestion metrics
- API metrics
- Exposes /metrics endpoint

### 8. Cache Package

- Redis integration for high-performance caching
- Last processed block caching
- Transaction hash deduplication
- Graceful degradation if Redis unavailable

### 9. Stream Package

- Real-time event streaming
- WebSocket and SSE support
- Event buffering for new clients
- Non-blocking publish

### 10. Logger Package

- Structured JSON and text logging
- File rotation with lumberjack
- Configurable log levels
- Dual output (stdout + file)

## Key Design Decisions

### 1. Multi-Provider Architecture

- **Provider Pool**: Manages multiple RPC providers with weighted selection
- **Circuit Breaker**: Prevents cascading failures, auto-recovery
- **Automatic Failover**: Seamlessly switches to healthy providers
- **Provider Metrics**: Track performance per provider
- **YAML Configuration**: Centralized provider management

### 2. Performance Optimizations

- **BulkWrite**: MongoDB batch inserts (~30% faster than InsertMany)
- **Block Header Cache**: Reduces redundant RPC calls by 60-80%
- **Parallel Fetching**: Concurrent block header requests (semaphore-limited)
- **Adaptive Batch Sizing**: Dynamically adjusts based on success/failure (can be disabled)
- **Redis Caching**: Sub-millisecond lookups for hot data
- **Ordered MongoDB Queries**: Uses `bson.D` for proper sort ordering

### 3. Event Processing

- **Polling Strategy**: Poll Ethereum node at configurable intervals
- **Adaptive Batching**: Batch size adjusts based on performance (can be disabled for fixed size)
- **Fixed Batch Mode**: When adaptive batch is disabled, uses `BLOCK_BATCH_SIZE` as fixed batch size
- **Block Range Processing**: Process blocks in batches respecting provider limits
- **Idempotency**: Composite index (tx_hash, log_index) prevents duplicates
- **Error Handling**: Retry logic with exponential backoff

### 4. Data Storage

- **MongoDB Collections**:
  - `transfers`: Normalized transfer events (Decimal128 value)
  - `processed_blocks`: Track last processed block (for idempotency)
- **Indexes**:
  - Composite unique: (tx_hash, log_index) for idempotency
  - Token address + block number (for fast queries)
  - From/To addresses (for user queries)
  - Timestamp (for time-based aggregations)
- **Redis Cache**:
  - `ethereum:last_block`: Last processed block number
  - `ethereum:tx:*`: Transaction hash deduplication (24h TTL)

### 5. API Design

- **RESTful Endpoints**:
  - `GET /api/v1/transfers` - Query transfers with filters
  - `GET /api/v1/aggregates` - Get aggregated statistics
  - `GET /metrics` - Prometheus metrics
  - `GET /health` - Health check
- **Streaming Endpoints** (optional):
  - `GET /ws` - WebSocket streaming
  - `GET /sse` - Server-Sent Events streaming
- **Query Parameters**: Filtering (token, address, date range), pagination

### 6. Observability

- **Metrics**:
  - Ingestion: transfers processed, blocks processed, errors
  - Provider: RPC requests, errors, duration per provider
  - API: HTTP requests, duration, status codes
- **Structured Logging**: JSON format with rotation
- **Health Endpoints**: Service health checks
- **Provider Tracking**: Logs which provider handled each request

### 7. DRY Principles

- Shared utilities for common operations
- Reusable components (parser, fetcher, client)
- Consistent error handling patterns
- Interface-based design for testability

## Configuration

### Environment Variables

**Core Configuration:**

- `ETH_RPC_URL` - Single RPC URL (legacy mode, optional if RPC_CONFIG set)
- `RPC_CONFIG` - Path to provider YAML config (preferred)
- `MONGODB_URI` - MongoDB connection string
- `MONGODB_DB` - Database name
- `START_BLOCK` - Starting block number
- `POLL_INTERVAL` - Polling interval in seconds
- `SERVER_PORT` - HTTP server port

**Redis Configuration:**

- `REDIS_URI` - Redis connection string
- `USE_REDIS` - Enable/disable Redis (default: true)

**Batch Configuration:**

- `BLOCK_BATCH_SIZE` - Initial/fixed batch size (default: 10)
- `ADAPTIVE_BATCH` - Enable adaptive sizing (default: true). Set to `false` to use fixed batch size
- `BATCH_MIN_SIZE` - Minimum batch size when adaptive (default: 1)
- `BATCH_MAX_SIZE` - Maximum batch size when adaptive (default: 100)
- `BATCH_SUCCESS_STREAK` - Successes before increasing (default: 3)
- `BATCH_FAILURE_BACKOFF` - Divisor on failure (default: 2)
- `RESET_START_BLOCK` - Force start from START_BLOCK (default: false)

**Note**: When `ADAPTIVE_BATCH=false`, the service uses `BLOCK_BATCH_SIZE` as a fixed batch size, ignoring min/max settings.

**Logging Configuration:**

- `LOG_LEVEL` - Log level (debug, info, warn, error)
- `LOG_TO_FILE` - Enable file logging (default: false)
- `LOG_FILE_PATH` - Log file path (default: logs/app.log)
- `LOG_FORMAT` - Format (text or json, default: text)
- `LOG_MAX_SIZE_MB` - Max file size (default: 100)
- `LOG_MAX_BACKUPS` - Max backup files (default: 7)
- `LOG_MAX_AGE_DAYS` - Max age of logs (default: 30)

**Streaming Configuration:**

- `ENABLE_STREAM` - Enable streaming (default: false)
- `STREAM_TYPE` - Type (ws or sse, default: ws)
- `STREAM_ROUTE` - Route path (default: /ws)
- `STREAM_BUFFER` - Buffer size (default: 1024)

### Provider YAML Configuration

See `config/providers.example.yaml` for provider configuration template.

## Docker Compose Services

1. **app**: Main application service
2. **mongodb**: MongoDB 7.0 with persistent volume
3. **redis**: Redis 7 with persistence and LRU eviction
4. **prometheus**: Metrics collection
5. **grafana**: Metrics visualization

## Error Handling Strategy

- Structured errors with context
- Graceful degradation (Redis unavailable → MongoDB fallback)
- Circuit breaker prevents cascading failures
- Retry mechanisms for transient failures
- Logging with appropriate levels
- HTTP error responses with proper status codes

## Testing Considerations

- Unit tests for each layer
- Integration tests for repository
- Mock Ethereum client for testing
- Test containers for MongoDB and Redis
- Provider pool failover testing
- Circuit breaker state machine testing
