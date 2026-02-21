# GoComet - Low Level Design (LLD)

## 1. Database Schema

### 1.1 Entity Relationship Diagram

```
┌────────────┐       ┌────────────┐       ┌────────────┐
│   users    │       │   rides    │       │  drivers   │
├────────────┤       ├────────────┤       ├────────────┤
│ id (PK)    │───────│ user_id    │       │ id (PK)    │
│ phone      │       │ driver_id  │───────│ phone      │
│ name       │       │ pickup_*   │       │ name       │
│ email      │       │ dropoff_*  │       │ vehicle_*  │
│ rating     │       │ status     │       │ status     │
│ created_at │       │ fare_*     │       │ rating     │
└────────────┘       │ created_at │       │ location_* │
                     └─────┬──────┘       └────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
        ┌─────┴─────┐           ┌───────┴──────┐
        │   trips   │           │ ride_offers  │
        ├───────────┤           ├──────────────┤
        │ id (PK)   │           │ id (PK)      │
        │ ride_id   │           │ ride_id      │
        │ driver_id │           │ driver_id    │
        │ user_id   │           │ status       │
        │ status    │           │ expires_at   │
        │ fare_*    │           └──────────────┘
        │ start_time│
        │ end_time  │
        └─────┬─────┘
              │
        ┌─────┴─────┐
        │ payments  │
        ├───────────┤
        │ id (PK)   │
        │ trip_id   │
        │ amount    │
        │ method    │
        │ status    │
        │ psp_*     │
        └───────────┘
```

### 1.2 Table Definitions

```sql
-- Core indexes for performance
CREATE INDEX idx_rides_user_status ON rides(user_id, status);
CREATE INDEX idx_rides_driver_status ON rides(driver_id, status);
CREATE INDEX idx_drivers_status_type ON drivers(status, vehicle_type);
CREATE INDEX idx_trips_ride ON trips(ride_id);
CREATE INDEX idx_payments_trip ON payments(trip_id);
```

## 2. API Specifications

### 2.1 REST Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /v1/users | Create user |
| GET | /v1/users/{id} | Get user |
| POST | /v1/drivers | Create driver |
| GET | /v1/drivers/{id} | Get driver |
| POST | /v1/drivers/{id}/location | Update location |
| POST | /v1/drivers/{id}/online | Go online |
| POST | /v1/drivers/{id}/offline | Go offline |
| POST | /v1/drivers/{id}/accept | Accept ride |
| GET | /v1/drivers/{id}/offers | Get pending offers |
| POST | /v1/rides | Create ride |
| GET | /v1/rides/{id} | Get ride |
| POST | /v1/rides/{id}/cancel | Cancel ride |
| GET | /v1/rides/{id}/track | SSE live tracking |
| POST | /v1/trips/start | Start trip |
| GET | /v1/trips/{id} | Get trip |
| POST | /v1/trips/{id}/end | End trip |
| POST | /v1/payments | Process payment |

### 2.2 Request/Response Formats

#### Create Ride
```json
// POST /v1/rides
// Request
{
    "user_id": "uuid",
    "pickup": {
        "lat": 12.9716,
        "lng": 77.5946,
        "address": "MG Road, Bangalore"
    },
    "dropoff": {
        "lat": 12.9352,
        "lng": 77.6245,
        "address": "Koramangala"
    },
    "vehicle_type": "sedan",
    "payment_method": "wallet"
}

// Response 201
{
    "id": "ride-uuid",
    "status": "matching",
    "estimated_fare": 250.00,
    "surge_multiplier": 1.2,
    "estimated_distance_km": 8.5,
    "estimated_duration_mins": 25
}
```

#### Update Location
```json
// POST /v1/drivers/{id}/location
// Request
{
    "lat": 12.9716,
    "lng": 77.5946,
    "heading": 45.0,
    "speed": 30.5
}

// Response 200
{
    "status": "ok",
    "timestamp": "2024-01-15T10:30:00Z"
}
```

## 3. State Machines

### 3.1 Ride State Machine

```
┌─────────┐
│ pending │
└────┬────┘
     │
     ▼
┌──────────┐     ┌───────────┐
│ matching │────▶│ cancelled │
└────┬─────┘     └───────────┘
     │                 ▲
     ▼                 │
┌─────────────────┐    │
│ driver_assigned │────┤
└────────┬────────┘    │
         │             │
         ▼             │
┌─────────────────┐    │
│ driver_arrived  │────┤
└────────┬────────┘    │
         │             │
         ▼             │
┌─────────────────┐    │
│  in_progress    │────┘
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   completed     │
└─────────────────┘
```

### 3.2 Valid Transitions

```go
var validRideTransitions = map[string][]string{
    "pending":          {"matching", "cancelled"},
    "matching":         {"driver_assigned", "cancelled"},
    "driver_assigned":  {"driver_arrived", "cancelled"},
    "driver_arrived":   {"in_progress", "cancelled"},
    "in_progress":      {"completed", "cancelled"},
    "completed":        {},
    "cancelled":        {},
}
```

## 4. Matching Algorithm

### 4.1 Algorithm Steps

```go
func FindBestDriver(ride *Ride) (*Driver, error) {
    // Step 1: Get nearby drivers (Redis GEORADIUS)
    nearbyDrivers := redis.GeoRadius(
        "drivers:locations:" + ride.VehicleType,
        ride.PickupLng, ride.PickupLat,
        5, "km",
        WITHDIST, COUNT(50), ASC
    )

    // Step 2: Filter candidates
    candidates := filterByStatus(nearbyDrivers, "online")
    candidates = filterByNoActiveRide(candidates)

    // Step 3: Score each candidate
    for _, driver := range candidates {
        score := 100.0
        score -= driver.Distance * 10  // Distance penalty
        score += driver.Rating * 5     // Rating bonus
        driver.Score = score
    }

    // Step 4: Sort by score
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].Score > candidates[j].Score
    })

    // Step 5: Return top candidate
    return candidates[0], nil
}
```

### 4.2 Scoring Factors

| Factor | Weight | Impact |
|--------|--------|--------|
| Distance | -10/km | Closer = Better |
| Rating | +5/star | Higher = Better |
| Total Trips | +10 (>1000) | Experienced = Better |
| Acceptance Rate | +10*rate | Higher = Better |

## 5. Caching Strategy

### 5.1 Redis Data Structures

```
# GeoHash for driver locations
GEOADD drivers:locations:sedan {lng} {lat} {driver_id}

# Driver metadata
HSET driver:{id}:meta status "online" vehicle_type "sedan" rating "4.8"

# Driver location details
SET driver:{id}:location '{"lat":12.97,"lng":77.59,"heading":45}' EX 300

# Active rides
SET driver:{id}:active_ride {ride_id} EX 3600
SET user:{id}:active_ride {ride_id} EX 3600

# Idempotency keys
SET idempotency:{key} '{"status":201,"body":{...}}' EX 86400

# Rate limiting
INCR ratelimit:{ip}:{endpoint}
EXPIRE ratelimit:{ip}:{endpoint} 60
```

### 5.2 Cache Invalidation

| Event | Action |
|-------|--------|
| Driver offline | Remove from GeoSet, update meta |
| Ride assigned | Update ride cache, set active ride |
| Trip completed | Clear active ride keys |
| Location update | Update GeoSet, update location key |

## 6. Fare Calculation

### 6.1 Fare Formula

```
Total = (BaseFare + DistanceFare + TimeFare) × SurgeMultiplier

Where:
- BaseFare = Fixed base (₹25-80 based on vehicle)
- DistanceFare = distance_km × rate_per_km
- TimeFare = duration_mins × rate_per_min
- SurgeMultiplier = 1.0 to 2.0
```

### 6.2 Vehicle Rates

| Type | Base | /km | /min | Min Fare |
|------|------|-----|------|----------|
| Auto | ₹25 | ₹12 | ₹1.0 | ₹30 |
| Mini | ₹40 | ₹14 | ₹1.2 | ₹50 |
| Sedan | ₹50 | ₹17 | ₹1.5 | ₹80 |
| SUV | ₹80 | ₹22 | ₹2.0 | ₹120 |

### 6.3 Surge Pricing

```go
func CalculateSurge(demand, supply int) float64 {
    ratio := float64(demand) / float64(supply)

    switch {
    case ratio < 1.0:  return 1.0
    case ratio < 1.5:  return 1.2
    case ratio < 2.0:  return 1.5
    case ratio < 3.0:  return 1.8
    default:           return 2.0
    }
}
```

## 7. Real-time Updates

### 7.1 SSE Implementation

```
Client connects to: GET /v1/rides/{id}/track

Server sends:
event: location
data: {"driver_id":"...", "lat":12.97, "lng":77.59, "eta_mins":5}

event: heartbeat
data: {"time":"2024-01-15T10:30:00Z"}

event: status
data: {"status":"driver_arrived"}
```

### 7.2 Pub/Sub Flow

```
1. Driver updates location
2. Service publishes to Redis channel "driver:location:updates"
3. SSE handler subscribes to channel
4. Broadcast to connected clients for that ride
```

## 8. Error Handling

### 8.1 Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| not_found | 404 | Resource not found |
| bad_request | 400 | Invalid input |
| conflict | 409 | Resource conflict |
| idempotency_conflict | 409 | Different request with same key |
| rate_limit_exceeded | 429 | Too many requests |
| no_drivers_available | 503 | No drivers in area |
| ride_already_assigned | 409 | Ride taken |
| offer_expired | 410 | Offer timed out |

### 8.2 Idempotency

```go
// Middleware checks idempotency key
1. Hash request body
2. Check Redis for existing response
3. If exists with same hash → return cached response
4. If exists with different hash → return 409 conflict
5. Otherwise → process request, cache response
```

## 9. Performance Optimizations

### 9.1 Database
- Connection pooling (25 connections)
- Prepared statements
- Appropriate indexes
- Read replicas for GET endpoints

### 9.2 Redis
- Pipeline batch operations
- Connection pooling (100 connections)
- GeoHash for O(log N) spatial queries

### 9.3 API
- Stateless handlers
- Concurrent request processing
- Efficient JSON serialization
- Response caching where applicable

## 10. Testing Strategy

### 10.1 Unit Tests
- Fare calculation
- State machine transitions
- Validation logic
- Matching algorithm

### 10.2 Integration Tests
- Full ride booking flow
- Payment processing
- Concurrent ride requests

### 10.3 Load Tests
- 1000+ location updates
- 100+ concurrent rides
- Sustained mixed load
