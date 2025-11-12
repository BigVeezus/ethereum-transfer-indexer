# Quick Start Guide

## Prerequisites

- Docker and Docker Compose installed
- Access to Ethereum RPC endpoint(s) (local node, Infura, Alchemy, etc.)
- Go 1.23+ (for local development)

## Option 1: Run with Docker Compose (Recommended)

This is the easiest way to run the entire stack:

### 1. Create Environment File

```bash
cp .env.example .env
```

### 2. Configure Environment Variables

Edit `.env` file with your settings:

**Required:**

```bash
# Either use single provider (legacy mode)
ETH_RPC_URL=https://your-ethereum-rpc-url

# OR use provider YAML config (recommended for production)
RPC_CONFIG=config/providers.yaml
```

**Provider Configuration (Recommended):**

Create `config/providers.yaml` based on `config/providers.example.yaml`:

```yaml
providers:
  - name: alchemy
    url: https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
    weight: 10
    maxRange: 10
    timeout: 30s

  - name: infura
    url: https://mainnet.infura.io/v3/YOUR_PROJECT_ID
    weight: 5
    maxRange: 10
    timeout: 30s

circuit_breaker:
  failure_threshold: 5
  success_threshold: 2
  timeout: 60s
  half_open_max_calls: 3
```

**Optional Configuration:**

```bash
# MongoDB
MONGODB_URI=mongodb://mongodb:27017
MONGODB_DB=ethereum

# Redis (optional but recommended)
REDIS_URI=redis://redis:6379
USE_REDIS=true

# Ingestion
START_BLOCK=0
POLL_INTERVAL=12
BLOCK_BATCH_SIZE=10
ADAPTIVE_BATCH=true
RESET_START_BLOCK=false

# Logging
LOG_LEVEL=info
LOG_TO_FILE=false
LOG_FORMAT=text

# Streaming (optional)
ENABLE_STREAM=false
STREAM_TYPE=ws
STREAM_ROUTE=/ws
```

### 3. Start All Services

```bash
# For newer Docker versions (Docker Desktop 4.0+)
docker compose up -d

# For older versions
docker-compose up -d
```

### 4. Check Logs

```bash
# View application logs
docker compose logs -f app

# View all services
docker compose logs -f
```

### 5. Access Services

- **API**: http://localhost:8080
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)
- **Health Check**: http://localhost:8080/health
- **Metrics**: http://localhost:8080/metrics

### 6. Stop Services

```bash
# For newer Docker versions
docker compose down

# For older versions
docker-compose down
```

## Option 2: Run Locally (Development)

### 1. Start Dependencies

```bash
# Start MongoDB, Redis, Prometheus, Grafana
docker compose up -d mongodb redis prometheus grafana
```

### 2. Create Environment File

Same as Option 1, but update connection strings for local access:

```bash
MONGODB_URI=mongodb://localhost:27017
REDIS_URI=redis://localhost:6379
```

### 3. Install Go Dependencies

```bash
go mod download
```

### 4. Run the Application

```bash
go run cmd/server/main.go
```

## Testing the API

### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{ "status": "healthy" }
```

### Get Transfers

```bash
# Get recent transfers
curl "http://localhost:8080/api/v1/transfers?limit=10"

# Filter by token address
curl "http://localhost:8080/api/v1/transfers?token=0xA0b86991c6218b36c1d19D4a2e9Eb0c3606eB48&limit=10"

# Filter by sender address
curl "http://localhost:8080/api/v1/transfers?from=0x...&limit=10"

# Filter by date range
curl "http://localhost:8080/api/v1/transfers?from_date=2024-01-01&to_date=2024-01-31"
```

### Get Aggregates

```bash
# Get aggregate statistics for a token
curl "http://localhost:8080/api/v1/aggregates?token=0xA0b86991c6218b36c1d19D4a2e9Eb0c3606eB48"

# Get aggregates for an address
curl "http://localhost:8080/api/v1/aggregates?from=0x..."
```

### View Metrics

```bash
curl http://localhost:8080/metrics
```

### Test Streaming (if enabled)

**WebSocket:**

```bash
# Using wscat (install: npm install -g wscat)
wscat -c ws://localhost:8080/ws
```

**SSE:**

```bash
curl -N http://localhost:8080/sse
```

## Configuration Options

### Core Settings

| Variable        | Description             | Default                     |
| --------------- | ----------------------- | --------------------------- |
| `ETH_RPC_URL`   | Single RPC URL (legacy) | -                           |
| `RPC_CONFIG`    | Path to provider YAML   | -                           |
| `MONGODB_URI`   | MongoDB connection      | `mongodb://localhost:27017` |
| `MONGODB_DB`    | Database name           | `ethereum`                  |
| `START_BLOCK`   | Starting block          | `0`                         |
| `POLL_INTERVAL` | Poll interval (seconds) | `12`                        |
| `SERVER_PORT`   | HTTP port               | `8080`                      |

### Redis Settings

| Variable    | Description      | Default                  |
| ----------- | ---------------- | ------------------------ |
| `REDIS_URI` | Redis connection | `redis://localhost:6379` |
| `USE_REDIS` | Enable Redis     | `true`                   |

### Batch Settings

| Variable                | Description                                    | Default |
| ----------------------- | ---------------------------------------------- | ------- |
| `BLOCK_BATCH_SIZE`      | Initial/fixed batch size                       | `10`    |
| `ADAPTIVE_BATCH`        | Enable adaptive sizing (set `false` for fixed) | `true`  |
| `BATCH_MIN_SIZE`        | Min batch size (only when adaptive enabled)    | `1`     |
| `BATCH_MAX_SIZE`        | Max batch size (only when adaptive enabled)    | `100`   |
| `BATCH_SUCCESS_STREAK`  | Successes before increase                      | `3`     |
| `BATCH_FAILURE_BACKOFF` | Divisor on failure                             | `2`     |
| `RESET_START_BLOCK`     | Force start from START_BLOCK                   | `false` |

**Note**: When `ADAPTIVE_BATCH=false`, the service uses `BLOCK_BATCH_SIZE` as a fixed batch size. Min/max settings are ignored.

### Logging Settings

| Variable           | Description         | Default        |
| ------------------ | ------------------- | -------------- |
| `LOG_LEVEL`        | Log level           | `info`         |
| `LOG_TO_FILE`      | Enable file logging | `false`        |
| `LOG_FILE_PATH`    | Log file path       | `logs/app.log` |
| `LOG_FORMAT`       | Format (text/json)  | `text`         |
| `LOG_MAX_SIZE_MB`  | Max file size (MB)  | `100`          |
| `LOG_MAX_BACKUPS`  | Max backup files    | `7`            |
| `LOG_MAX_AGE_DAYS` | Max age (days)      | `30`           |

### Streaming Settings

| Variable        | Description      | Default |
| --------------- | ---------------- | ------- |
| `ENABLE_STREAM` | Enable streaming | `false` |
| `STREAM_TYPE`   | Type (ws/sse)    | `ws`    |
| `STREAM_ROUTE`  | Route path       | `/ws`   |
| `STREAM_BUFFER` | Buffer size      | `1024`  |

## Important Notes

### Provider Configuration

**For Production:**

- Use `RPC_CONFIG` with YAML file for multi-provider failover
- Configure at least 2-3 providers for redundancy
- Set appropriate `maxRange` based on provider tier limits
- Use higher `weight` for preferred providers

**For Development:**

- Single provider via `ETH_RPC_URL` is sufficient
- Free-tier providers typically limit `maxRange` to 10 blocks

### Block Range Limits

Free-tier RPC providers (Alchemy, Infura) typically limit `eth_getLogs` to 10 blocks. The service:

- Respects `BLOCK_BATCH_SIZE` configuration
- Automatically adjusts batch size based on provider limits (when adaptive enabled)
- Uses adaptive batching to optimize throughput (can be disabled for fixed size)
- When `ADAPTIVE_BATCH=false`, uses `BLOCK_BATCH_SIZE` as fixed batch size

### Starting Block

- By default, service resumes from last processed block in MongoDB
- Set `RESET_START_BLOCK=true` to force start from `START_BLOCK`
- Useful when changing `START_BLOCK` after initial run

### Redis

- Redis is optional but recommended for performance
- Service gracefully degrades to MongoDB-only if Redis unavailable
- Last processed block cached in Redis for fast lookups
- Transaction hash deduplication uses Redis with 24h TTL

## Troubleshooting

### Service Won't Start

**Check RPC Configuration:**

- Verify `ETH_RPC_URL` or `RPC_CONFIG` is set correctly
- Test RPC endpoint: `curl -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $ETH_RPC_URL`
- Check provider YAML syntax if using `RPC_CONFIG`

**Check Dependencies:**

- Verify MongoDB is running: `docker compose ps mongodb`
- Verify Redis is running: `docker compose ps redis`
- Check logs: `docker compose logs mongodb redis`

**Check Logs:**

```bash
docker compose logs app
```

### No Transfers Being Ingested

**Verify Configuration:**

- Check `START_BLOCK` (might be too high for current chain)
- Verify RPC endpoint is accessible and returns data
- Check `BLOCK_BATCH_SIZE` (free tier limit is typically 10)

**Check Logs:**

```bash
docker compose logs -f app | grep -i "error\|failed\|processed"
```

**Verify Provider:**

- Check which provider is being used in logs
- Verify provider health in metrics: `curl http://localhost:8080/metrics | grep rpc_`

### MongoDB Connection Errors

**Check MongoDB:**

```bash
# Verify container is running
docker compose ps mongodb

# Check MongoDB logs
docker compose logs mongodb

# Test connection
docker compose exec mongodb mongosh --eval "db.adminCommand('ping')"
```

**Verify Connection String:**

- Docker: `mongodb://mongodb:27017`
- Local: `mongodb://localhost:27017`

### Redis Connection Errors

**Check Redis:**

```bash
# Verify container is running
docker compose ps redis

# Check Redis logs
docker compose logs redis

# Test connection
docker compose exec redis redis-cli ping
```

**Note:** Service continues without Redis (graceful degradation)

### Provider Failures

**Check Provider Health:**

```bash
# View provider metrics
curl http://localhost:8080/metrics | grep rpc_errors_total
```

**Common Issues:**

- Rate limiting: Reduce `BLOCK_BATCH_SIZE` or add more providers
- Network issues: Check provider URLs are accessible
- Circuit breaker: Provider marked unhealthy after failures, auto-recovers after timeout

### High Memory Usage

**Optimize Batch Size:**

- Reduce `BATCH_MAX_SIZE` if memory constrained
- Enable adaptive batching to auto-adjust

**Check Cache:**

- Block header cache uses minimal memory (5min TTL)
- Redis cache has LRU eviction configured

## Monitoring

### Prometheus Metrics

Access metrics at: http://localhost:8080/metrics

Key metrics to monitor:

- `eth_transfers_processed_total` - Ingestion rate
- `rpc_errors_total` - Provider errors
- `rpc_request_duration_seconds` - Provider latency
- `http_requests_total` - API usage

### Grafana Dashboards

Access Grafana at: http://localhost:3000

Pre-configured:

- Prometheus datasource
- Create custom dashboards for:
  - Ingestion rate
  - Provider health
  - API performance
  - Error rates

### Logs

**File Logs (if enabled):**

```bash
tail -f logs/app.log
```

**Docker Logs:**

```bash
docker compose logs -f app
```

## Next Steps

1. **Configure Providers**: Set up multiple RPC providers in YAML for redundancy
2. **Monitor Metrics**: Set up Grafana dashboards for observability
3. **Enable Streaming**: Set `ENABLE_STREAM=true` for real-time events
4. **Tune Batch Size**: Adjust `BATCH_MIN_SIZE` and `BATCH_MAX_SIZE` based on provider limits
5. **Enable File Logging**: Set `LOG_TO_FILE=true` for production
6. **Review Indexes**: Verify MongoDB indexes are created (check logs on startup)

## Production Checklist

- [ ] Configure multiple RPC providers in YAML
- [ ] Set appropriate `maxRange` for each provider
- [ ] Enable Redis for performance
- [ ] Configure file logging with rotation
- [ ] Set up Grafana dashboards
- [ ] Configure monitoring alerts
- [ ] Review and adjust batch size limits
- [ ] Test failover scenarios
- [ ] Set up log aggregation (if using JSON format)
- [ ] Review security settings (API keys, network access)
