package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	IdempotencyHeader  = "Idempotency-Key"
	idempotencyTTL     = 24 * time.Hour
	idempotencyPrefix  = "idempotency:"
)

type IdempotencyMiddleware struct {
	redis *redis.Client
}

type cachedResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	BodyHash   string            `json:"body_hash"`
}

func NewIdempotencyMiddleware(redisClient *redis.Client) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{redis: redisClient}
}

// responseWriter captures the response for caching
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

func (m *IdempotencyMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to POST, PUT, PATCH methods
		if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}

		idempotencyKey := r.Header.Get(IdempotencyHeader)
		if idempotencyKey == "" {
			// No idempotency key, proceed normally
			next.ServeHTTP(w, r)
			return
		}

		// Read and hash the request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		bodyHash := hashBody(bodyBytes)
		cacheKey := idempotencyPrefix + idempotencyKey

		ctx := r.Context()

		// Check if we have a cached response
		cached, err := m.getCachedResponse(ctx, cacheKey)
		if err == nil {
			// Verify the body hash matches
			if cached.BodyHash != bodyHash {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "idempotency_conflict",
					"message": "idempotency key already used with different request",
				})
				return
			}

			// Return cached response
			for k, v := range cached.Headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(cached.StatusCode)
			w.Write(cached.Body)
			return
		}

		// Try to acquire lock for this idempotency key
		lockKey := cacheKey + ":lock"
		locked, err := m.redis.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
		if err != nil || !locked {
			// Another request is processing
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "request_in_progress",
				"message": "a request with this idempotency key is already being processed",
			})
			return
		}
		defer m.redis.Del(ctx, lockKey)

		// Capture the response
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		// Cache successful responses (2xx)
		if rw.statusCode >= 200 && rw.statusCode < 300 {
			headers := make(map[string]string)
			headers["Content-Type"] = rw.Header().Get("Content-Type")

			cached := cachedResponse{
				StatusCode: rw.statusCode,
				Headers:    headers,
				Body:       rw.body.Bytes(),
				BodyHash:   bodyHash,
			}

			data, _ := json.Marshal(cached)
			m.redis.Set(ctx, cacheKey, data, idempotencyTTL)
		}
	})
}

func (m *IdempotencyMiddleware) getCachedResponse(ctx context.Context, key string) (*cachedResponse, error) {
	data, err := m.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var cached cachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}

	return &cached, nil
}

func hashBody(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}
