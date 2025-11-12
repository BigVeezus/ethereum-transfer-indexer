# Ethereum ERC-20 Transfer Event Ingestion Service

A scalable Go service that ingests Ethereum ERC-20 Transfer events, stores them in MongoDB, and exposes Prometheus metrics and a REST API for querying aggregated data.

## Features

- **Event Ingestion**: Continuously polls Ethereum node for ERC-20 Transfer events
- **Multi-Provider Failover**: Automatic failover across multiple RPC providers with circuit breaker
- **Data Storage**: Normalized event storage in MongoDB with optimized indexes (Decimal128 for precision)
- **Redis Caching**: High-performance caching for last processed block and deduplication
- **REST API**: Query transfers and aggregated statistics via HTTP
- **Real-Time Streaming**: Optional WebSocket/SSE streaming for live transfer events
- **Adaptive Batch Sizing**: Automatically adjusts batch size based on performance (can be disabled)
- **Metrics**: Comprehensive Prometheus metrics including provider-level tracking
- **Structured Logging**: JSON/text logging with file rotation
- **Docker Compose**: Complete stack with MongoDB, Redis, Prometheus, and Grafana

## Architecture

The service follows a modular, scalable architecture:

- **Ethereum Layer**: Multi-provider pool with circuit breaker, fetcher with caching, and parser
- **Repository Layer**: MongoDB operations with BulkWrite optimization and Redis caching
- **Service Layer**: Business logic with adaptive batch sizing and stream integration
- **Handler Layer**: HTTP request handling and WebSocket/SSE streaming
- **Metrics Layer**: Comprehensive Prometheus metrics including provider-level tracking
- **Cache Layer**: Redis for high-performance lookups with graceful degradation
- **Stream Layer**: Real-time event streaming with buffering

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Access to an Ethereum RPC endpoint (local node or Infura/Alchemy)

## Quick Start

1. **Clone and configure**:

```bash
cp .env.example .env
# Edit .env with your Ethereum RPC URL
```

2. **Start services**:

```bash
docker-compose up -d
```

3. **Access services**:

- API: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Configuration

See `.env.example` for complete configuration options. Key settings:

**Ethereum RPC:**

- `ETH_RPC_URL`: Single RPC URL (legacy mode)
- `RPC_CONFIG`: Path to provider YAML config (recommended for production)

**Database:**

- `MONGODB_URI`: MongoDB connection string
- `MONGODB_DB`: Database name
- `REDIS_URI`: Redis connection string
- `USE_REDIS`: Enable Redis caching (default: true)

**Ingestion:**

- `START_BLOCK`: Starting block number
- `POLL_INTERVAL`: Polling interval in seconds
- `BLOCK_BATCH_SIZE`: Initial/fixed batch size
- `ADAPTIVE_BATCH`: Enable adaptive batch sizing (default: true)

**Logging:**

- `LOG_LEVEL`: Log level (debug, info, warn, error)
- `LOG_TO_FILE`: Enable file logging
- `LOG_FORMAT`: Format (text or json)

**Streaming (Optional):**

- `ENABLE_STREAM`: Enable WebSocket/SSE streaming
- `STREAM_TYPE`: Type (ws or sse)

See [QUICKSTART.md](QUICKSTART.md) for detailed configuration guide.

## API Endpoints

### Get Transfers

```
GET /api/v1/transfers
```

Query parameters:

- `token`: Filter by token address
- `from`: Filter by sender address
- `to`: Filter by recipient address
- `start_block`: Minimum block number
- `end_block`: Maximum block number
- `start_time`: Start time (RFC3339)
- `end_time`: End time (RFC3339)
- `limit`: Results per page (default: 100, max: 1000)
- `offset`: Pagination offset

Example:

```bash
curl "http://localhost:8080/api/v1/transfers?token=0x...&limit=10"
```

### Get Aggregates

```
GET /api/v1/aggregates
```

Returns aggregated statistics with same filter parameters as transfers endpoint.

Example:

```bash
curl "http://localhost:8080/api/v1/aggregates?token=0x..."
```

### Health Check

```
GET /health
```

### Metrics

```
GET /metrics
```

Prometheus-compatible metrics endpoint.

## Development

### Local Development

1. **Start dependencies**:

```bash
docker-compose up -d mongodb prometheus grafana
```

2. **Run application**:

```bash
go mod download
go run cmd/server/main.go
```

### Building

```bash
go build -o server ./cmd/server
```

### Testing

```bash
go test ./...
```

## Monitoring

### Prometheus Metrics

**Ingestion Metrics:**

- `eth_transfers_processed_total`: Total transfers processed
- `eth_transfers_processing_duration_seconds`: Processing time
- `eth_blocks_processed_total`: Blocks processed
- `eth_ingestion_errors_total`: Error count

**Provider Metrics:**

- `rpc_requests_total`: RPC request count by provider and method
- `rpc_errors_total`: RPC error count by provider
- `rpc_request_duration_seconds`: RPC request latency by provider

**API Metrics:**

- `http_requests_total`: HTTP request count
- `http_request_duration_seconds`: HTTP request latency

### Grafana Dashboards

Import the provided Grafana dashboard configuration from `grafana/provisioning/` to visualize metrics.

## Scaling Considerations

- **Horizontal Scaling**: Stateless HTTP handlers allow multiple instances
- **Database Indexing**: Optimized indexes for common query patterns
- **Adaptive Batch Processing**: Automatically adjusts batch size (or fixed size if disabled)
- **Multi-Provider Failover**: Automatic failover prevents single point of failure
- **Redis Caching**: Sub-millisecond lookups for hot data
- **BulkWrite Optimization**: ~30% faster than InsertMany
- **Block Header Caching**: Reduces redundant RPC calls by 60-80%
- **Idempotency**: Tracks processed blocks to prevent duplicate ingestion

## Error Handling

- **Circuit Breaker**: Prevents cascading failures across RPC providers
- **Automatic Failover**: Seamlessly switches to healthy providers
- **Retry Logic**: Exponential backoff for transient failures
- **Graceful Degradation**: Redis unavailable → MongoDB fallback
- **Structured Error Logging**: JSON/text format with context
- **HTTP Error Responses**: Proper status codes and error messages

## License

MIT
