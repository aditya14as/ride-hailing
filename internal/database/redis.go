package database

import (
	"context"
	"time"

	"github.com/newrelic/go-agent/v3/integrations/nrredis-v9"
	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	*redis.Client
}

func NewRedis(addr, password string) (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Add New Relic instrumentation
	client.AddHook(nrredis.NewHook(nil))

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisDB{Client: client}, nil
}

func (r *RedisDB) Close() error {
	return r.Client.Close()
}

func (r *RedisDB) Health(ctx context.Context) error {
	return r.Ping(ctx).Err()
}
