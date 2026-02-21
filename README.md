# GoComet - Ride Hailing Platform

A multi-tenant, real-time ride-hailing system built with Go, PostgreSQL, and Redis.

## Features

- Real-time driver location tracking (SSE)
- Driver-rider matching within 1s p95
- Surge pricing based on demand/supply
- Trip lifecycle management
- Payment processing
- Idempotent APIs
- Rate limiting
- New Relic APM integration

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Make

### 1. Start Infrastructure

```bash
make docker-up
```

### 2. Run Migrations

```bash
make migrate-up
```

### 3. Seed Test Data (Optional)

```bash
go run scripts/seed_data.go
```

### 4. Start Server

```bash
make run
```

### 5. Open Frontend

Navigate to: http://localhost:8080

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /v1/users | Create user |
| POST | /v1/drivers | Create driver |
| POST | /v1/rides | Create ride |
| GET | /v1/rides/{id} | Get ride |
| POST | /v1/drivers/{id}/location | Update location |
| POST | /v1/drivers/{id}/accept | Accept ride |
| POST | /v1/trips/{id}/end | End trip |
| POST | /v1/payments | Process payment |
| GET | /v1/rides/{id}/track | SSE live tracking |

## Performance

Load test results on MacBook:

| Metric | Value |
|--------|-------|
| Location update latency | 14.7ms avg |
| Ride creation latency | 7.6ms avg |
| Throughput (mixed) | 197 req/s |
| Success rate | 100% |

## Project Structure

```
go-comet/
├── cmd/server/          # Application entry point
├── internal/
│   ├── config/          # Configuration
│   ├── database/        # DB connections
│   ├── models/          # Data models
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic
│   ├── handler/         # HTTP handlers
│   ├── middleware/      # Middleware (auth, rate limit, etc.)
│   └── cache/           # Redis caching
├── frontend/            # Web UI
├── migrations/          # SQL migrations
├── scripts/             # Utility scripts
└── docs/                # Documentation (HLD/LLD)
```

## New Relic Setup

1. Sign up at newrelic.com (free tier)
2. Get your license key
3. Update `.env`:
   ```
   NEW_RELIC_ENABLED=true
   NEW_RELIC_LICENSE_KEY=your_key
   ```
4. Restart server

## Running Load Tests

```bash
go run scripts/loadtest.go
```

## Documentation

- [High Level Design](docs/HLD.md)
- [Low Level Design](docs/LLD.md)

## Tech Stack

- **Language**: Go 1.21
- **Router**: Chi v5
- **Database**: PostgreSQL 15
- **Cache**: Redis 7
- **Monitoring**: New Relic
- **Frontend**: HTML/CSS/JS + TailwindCSS + Leaflet

## License

MIT
