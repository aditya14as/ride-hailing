package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	driverLocationKeyPrefix = "drivers:locations:"
	driverMetaKeyPrefix     = "driver:meta:"
	driverActiveRideKey     = "driver:active:"
	userActiveRideKey       = "user:active:"
	locationTTL             = 5 * time.Minute
)

type DriverLocation struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Heading   float64 `json:"heading,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	Accuracy  float64 `json:"accuracy,omitempty"`
	UpdatedAt int64   `json:"updated_at"`
}

type DriverLocationCache interface {
	UpdateLocation(ctx context.Context, driverID string, lat, lng float64, heading, speed, accuracy *float64) error
	GetDriverLocation(ctx context.Context, driverID string) (*DriverLocation, error)
	GetNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64, vehicleType string) ([]DriverWithDistance, error)
	RemoveDriver(ctx context.Context, driverID, vehicleType string) error
	SetDriverMeta(ctx context.Context, driverID, status, vehicleType string, rating float64) error
	GetDriverMeta(ctx context.Context, driverID string) (map[string]string, error)
	SetActiveRide(ctx context.Context, driverID, rideID string) error
	GetActiveRide(ctx context.Context, driverID string) (string, error)
	ClearActiveRide(ctx context.Context, driverID string) error
	SetUserActiveRide(ctx context.Context, userID, rideID string) error
	GetUserActiveRide(ctx context.Context, userID string) (string, error)
	ClearUserActiveRide(ctx context.Context, userID string) error
}

type DriverWithDistance struct {
	DriverID string
	Distance float64
}

type driverLocationCache struct {
	redis *redis.Client
}

func NewDriverLocationCache(redisClient *redis.Client) DriverLocationCache {
	return &driverLocationCache{redis: redisClient}
}

func (c *driverLocationCache) UpdateLocation(ctx context.Context, driverID string, lat, lng float64, heading, speed, accuracy *float64) error {
	// First, get driver's vehicle type from meta
	meta, err := c.GetDriverMeta(ctx, driverID)
	if err != nil {
		return err
	}

	vehicleType := meta["vehicle_type"]
	if vehicleType == "" {
		vehicleType = "sedan" // default
	}

	// Add to geo set for the vehicle type
	geoKey := driverLocationKeyPrefix + vehicleType
	if err := c.redis.GeoAdd(ctx, geoKey, &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err(); err != nil {
		return err
	}

	// Store detailed location info
	loc := DriverLocation{
		Lat:       lat,
		Lng:       lng,
		UpdatedAt: time.Now().Unix(),
	}
	if heading != nil {
		loc.Heading = *heading
	}
	if speed != nil {
		loc.Speed = *speed
	}
	if accuracy != nil {
		loc.Accuracy = *accuracy
	}

	locJSON, err := json.Marshal(loc)
	if err != nil {
		return err
	}

	locKey := driverMetaKeyPrefix + driverID + ":location"
	return c.redis.Set(ctx, locKey, locJSON, locationTTL).Err()
}

func (c *driverLocationCache) GetDriverLocation(ctx context.Context, driverID string) (*DriverLocation, error) {
	locKey := driverMetaKeyPrefix + driverID + ":location"
	data, err := c.redis.Get(ctx, locKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var loc DriverLocation
	if err := json.Unmarshal(data, &loc); err != nil {
		return nil, err
	}

	return &loc, nil
}

func (c *driverLocationCache) GetNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64, vehicleType string) ([]DriverWithDistance, error) {
	geoKey := driverLocationKeyPrefix + vehicleType

	locations, err := c.redis.GeoRadius(ctx, geoKey, lng, lat, &redis.GeoRadiusQuery{
		Radius:    radiusKm,
		Unit:      "km",
		WithDist:  true,
		WithCoord: true,
		Count:     50,
		Sort:      "ASC",
	}).Result()
	if err != nil {
		return nil, err
	}

	result := make([]DriverWithDistance, 0, len(locations))
	for _, loc := range locations {
		// Check if driver is online
		meta, err := c.GetDriverMeta(ctx, loc.Name)
		if err != nil {
			continue
		}
		if meta["status"] != "online" {
			continue
		}

		result = append(result, DriverWithDistance{
			DriverID: loc.Name,
			Distance: loc.Dist,
		})
	}

	return result, nil
}

func (c *driverLocationCache) RemoveDriver(ctx context.Context, driverID, vehicleType string) error {
	geoKey := driverLocationKeyPrefix + vehicleType
	return c.redis.ZRem(ctx, geoKey, driverID).Err()
}

func (c *driverLocationCache) SetDriverMeta(ctx context.Context, driverID, status, vehicleType string, rating float64) error {
	metaKey := driverMetaKeyPrefix + driverID
	return c.redis.HSet(ctx, metaKey, map[string]interface{}{
		"status":       status,
		"vehicle_type": vehicleType,
		"rating":       fmt.Sprintf("%.1f", rating),
	}).Err()
}

func (c *driverLocationCache) GetDriverMeta(ctx context.Context, driverID string) (map[string]string, error) {
	metaKey := driverMetaKeyPrefix + driverID
	return c.redis.HGetAll(ctx, metaKey).Result()
}

func (c *driverLocationCache) SetActiveRide(ctx context.Context, driverID, rideID string) error {
	key := driverActiveRideKey + driverID
	return c.redis.Set(ctx, key, rideID, time.Hour).Err()
}

func (c *driverLocationCache) GetActiveRide(ctx context.Context, driverID string) (string, error) {
	key := driverActiveRideKey + driverID
	result, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

func (c *driverLocationCache) ClearActiveRide(ctx context.Context, driverID string) error {
	key := driverActiveRideKey + driverID
	return c.redis.Del(ctx, key).Err()
}

func (c *driverLocationCache) SetUserActiveRide(ctx context.Context, userID, rideID string) error {
	key := userActiveRideKey + userID
	return c.redis.Set(ctx, key, rideID, time.Hour).Err()
}

func (c *driverLocationCache) GetUserActiveRide(ctx context.Context, userID string) (string, error) {
	key := userActiveRideKey + userID
	result, err := c.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

func (c *driverLocationCache) ClearUserActiveRide(ctx context.Context, userID string) error {
	key := userActiveRideKey + userID
	return c.redis.Del(ctx, key).Err()
}

// ParseRating parses rating string to float64
func ParseRating(ratingStr string) float64 {
	if ratingStr == "" {
		return 5.0
	}
	rating, err := strconv.ParseFloat(ratingStr, 64)
	if err != nil {
		return 5.0
	}
	return rating
}
