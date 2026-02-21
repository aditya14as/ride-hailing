package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	redis    *redis.Client
	requests int
	window   time.Duration
}

func NewRateLimiter(redisClient *redis.Client, requests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:    redisClient,
		requests: requests,
		window:   window,
	}
}

func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}

		key := fmt.Sprintf("ratelimit:%s:%s", clientIP, r.URL.Path)
		ctx := r.Context()

		allowed, remaining, err := rl.isAllowed(ctx, key)
		if err != nil {
			// On error, allow the request
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.requests))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "rate_limit_exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) isAllowed(ctx context.Context, key string) (bool, int, error) {
	pipe := rl.redis.Pipeline()

	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return true, rl.requests, err
	}

	count := int(incr.Val())
	remaining := rl.requests - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= rl.requests, remaining, nil
}
