# GoComet - High Level Design (HLD)

## 1. System Overview

GoComet is a multi-tenant ride-hailing platform designed to handle:
- Real-time driver location updates (1-2 per second)
- Driver-rider matching within 1s p95
- Trip lifecycle management
- Payment processing
- Live tracking via SSE

## 2. Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                  CLIENTS                                         │
│                                                                                  │
│     ┌──────────────┐      ┌──────────────┐      ┌──────────────┐                │
│     │  Rider App   │      │  Driver App  │      │   Admin UI   │                │
│     │   (React)    │      │   (React)    │      │   (React)    │                │
│     └──────┬───────┘      └──────┬───────┘      └──────┬───────┘                │
│            │                     │                     │                         │
└────────────┼─────────────────────┼─────────────────────┼────────────────────────┘
             │                     │                     │
             ▼                     ▼                     ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                             LOAD BALANCER                                        │
│                           (Nginx / ALB)                                          │
└──────────────────────────────────┬──────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           API GATEWAY (Go Chi)                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │    Rate     │  │   Auth      │  │ Idempotency │  │  New Relic  │            │
│  │  Limiting   │  │ Middleware  │  │  Middleware │  │  Middleware │            │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘            │
└──────────────────────────────────┬──────────────────────────────────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         │                         │                         │
         ▼                         ▼                         ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Ride Service   │     │ Driver Service  │     │ Payment Service │
│                 │     │                 │     │                 │
│  - Create Ride  │     │ - Location Upd  │     │ - Process Pay   │
│  - Get Ride     │     │ - Accept Ride   │     │ - Refunds       │
│  - Cancel Ride  │     │ - Go Online     │     │ - Receipts      │
│                 │     │                 │     │                 │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │              ┌────────┴────────┐              │
         │              │                 │              │
         │              │ Matching Engine │              │
         │              │                 │              │
         │              │ - Find Drivers  │              │
         │              │ - Score & Rank  │              │
         │              │ - Create Offers │              │
         │              │                 │              │
         │              └────────┬────────┘              │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              DATA LAYER                                          │
│                                                                                  │
│  ┌──────────────────────────────┐    ┌──────────────────────────────┐          │
│  │                              │    │                              │          │
│  │        PostgreSQL            │    │          Redis               │          │
│  │                              │    │                              │          │
│  │  - Users                     │    │  - Driver Locations (Geo)    │          │
│  │  - Drivers                   │    │  - Driver Metadata           │          │
│  │  - Rides                     │    │  - Active Rides Cache        │          │
│  │  - Trips                     │    │  - Idempotency Keys          │          │
│  │  - Payments                  │    │  - Rate Limiting             │          │
│  │  - Ride Offers               │    │  - Pub/Sub (SSE)             │          │
│  │                              │    │                              │          │
│  └──────────────────────────────┘    └──────────────────────────────┘          │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 3. Key Components

### 3.1 API Gateway
- Built with Go Chi router
- Middleware chain: Recovery → Logger → CORS → NewRelic → RateLimit → Idempotency

### 3.2 Services
| Service | Responsibility |
|---------|----------------|
| Ride Service | Ride creation, status management, cancellation |
| Driver Service | Location updates, status management, ride acceptance |
| Trip Service | Trip lifecycle, fare calculation |
| Payment Service | Payment processing, refunds |
| Matching Service | Driver-rider matching algorithm |
| Pricing Service | Fare estimation, surge calculation |

### 3.3 Data Storage
| Store | Use Case |
|-------|----------|
| PostgreSQL | Transactional data (users, rides, payments) |
| Redis | Caching, geo-queries, pub/sub, rate limiting |

## 4. Core Flows

### 4.1 Ride Booking Flow
```
1. User requests ride → Create Ride (status: pending)
2. Calculate surge → Update estimated fare
3. Find nearby drivers → Query Redis GeoHash
4. Score drivers → Create ride offers
5. Driver accepts → Assign driver (status: driver_assigned)
6. Driver arrives → Update status
7. Start trip → Create trip record
8. End trip → Calculate actual fare
9. Process payment → Complete ride
```

### 4.2 Location Update Flow
```
1. Driver sends location (every 1-2s)
2. Update Redis GeoHash (O(log N))
3. Store metadata in Redis Hash
4. If on active trip → Publish to Pub/Sub
5. SSE clients receive real-time updates
```

## 5. Scalability Considerations

### 5.1 Horizontal Scaling
- Stateless API servers (can scale to N instances)
- Redis Cluster for geo-queries
- PostgreSQL read replicas for read-heavy queries

### 5.2 Performance Targets
| Metric | Target | Achieved |
|--------|--------|----------|
| Location update latency | <50ms p95 | 14ms avg |
| Ride creation latency | <100ms p95 | 7.6ms avg |
| Driver matching | <1s p95 | ~100ms |
| Throughput | 10k req/min | ~11.8k req/min |

### 5.3 Bottleneck Mitigation
- Redis GeoHash for O(log N) spatial queries
- Connection pooling for database
- Idempotency caching to prevent duplicate processing
- Rate limiting per IP/user

## 6. Reliability

### 6.1 Fault Tolerance
- Graceful shutdown handling
- Idempotency for retry safety
- Database transactions for consistency
- Redis SET NX for distributed locks

### 6.2 Monitoring
- New Relic APM integration
- Custom metrics for business KPIs
- Alert thresholds for latency/errors

## 7. Security

- Rate limiting (100 req/min per IP)
- Input validation on all endpoints
- Idempotency key validation
- CORS configuration
- No SQL injection (parameterized queries)

## 8. Technology Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.21+ |
| Router | Chi v5 |
| Database | PostgreSQL 15 |
| Cache | Redis 7 |
| Monitoring | New Relic |
| Containerization | Docker |
