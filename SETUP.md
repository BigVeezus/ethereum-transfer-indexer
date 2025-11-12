# Setup Instructions

## Option 1: Install Docker (Recommended)

### macOS

1. **Download Docker Desktop for Mac**:

   - Visit: https://www.docker.com/products/docker-desktop/
   - Download and install Docker Desktop
   - Launch Docker Desktop and wait for it to start

2. **Verify installation**:

```bash
docker --version
docker compose version
```

3. **Run the service**:

```bash
cp .env.example .env
# Edit .env and set your ETH_RPC_URL
docker compose up -d
```

**Note**: Newer Docker versions use `docker compose` (without hyphen) instead of `docker-compose`.

## Option 2: Run Locally Without Docker

If you don't want to install Docker, you can run the service locally but you'll need to install MongoDB separately.

### Prerequisites

1. **Install MongoDB**:

```bash
# macOS with Homebrew
brew tap mongodb/brew
brew install mongodb-community

# Start MongoDB
brew services start mongodb-community
```

2. **Install Go** (if not already installed):

```bash
# macOS with Homebrew
brew install go
```

### Run the Service

1. **Create `.env` file**:

```bash
cp .env.example .env
```

2. **Edit `.env`** and set:

```bash
ETH_RPC_URL=your-ethereum-rpc-url
MONGODB_URI=mongodb://localhost:27017
MONGODB_DB=ethereum
START_BLOCK=0
POLL_INTERVAL=12
SERVER_PORT=8080
LOG_LEVEL=info
```

3. **Install dependencies**:

```bash
go mod download
```

4. **Run the application**:

```bash
go run cmd/server/main.go
```

The service will start on http://localhost:8080

### Optional: Install Prometheus and Grafana

If you want metrics visualization without Docker:

1. **Install Prometheus**:

```bash
brew install prometheus
```

2. **Start Prometheus** (edit prometheus.yml to point to localhost:8080):

```bash
prometheus --config.file=prometheus.yml
```

3. **Install Grafana**:

```bash
brew install grafana
```

4. **Start Grafana**:

```bash
brew services start grafana
# Access at http://localhost:3000 (admin/admin)
```

## Quick Test

Once running, test the API:

```bash
# Health check
curl http://localhost:8080/health

# Get transfers
curl "http://localhost:8080/api/v1/transfers?limit=10"
```
