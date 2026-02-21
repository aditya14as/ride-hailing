package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	"github.com/aditya/go-comet/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

type SSEHandler struct {
	rideRepo    repository.RideRepository
	driverCache cache.DriverLocationCache
	redis       *redis.Client
	clients     map[string]map[chan []byte]bool // rideID -> clients
	mu          sync.RWMutex
}

func NewSSEHandler(rideRepo repository.RideRepository, driverCache cache.DriverLocationCache, redisClient *redis.Client) *SSEHandler {
	handler := &SSEHandler{
		rideRepo:    rideRepo,
		driverCache: driverCache,
		redis:       redisClient,
		clients:     make(map[string]map[chan []byte]bool),
	}

	// Start Redis pub/sub listener
	go handler.startPubSubListener()

	return handler
}

func (h *SSEHandler) RegisterRoutes(r chi.Router) {
	r.Get("/rides/{id}/track", h.TrackRide)
}

// TrackRide handles SSE connections for real-time ride tracking
func (h *SSEHandler) TrackRide(w http.ResponseWriter, r *http.Request) {
	rideID := chi.URLParam(r, "id")
	if rideID == "" {
		http.Error(w, "ride id required", http.StatusBadRequest)
		return
	}

	// Verify ride exists and is trackable
	ride, err := h.rideRepo.GetByID(r.Context(), rideID)
	if err != nil || ride == nil {
		http.Error(w, "ride not found", http.StatusNotFound)
		return
	}

	if ride.DriverID == nil {
		http.Error(w, "no driver assigned yet", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan []byte, 10)

	// Register client
	h.registerClient(rideID, clientChan)
	defer h.unregisterClient(rideID, clientChan)

	// Flush initial response
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial location
	if loc, err := h.driverCache.GetDriverLocation(r.Context(), *ride.DriverID); err == nil && loc != nil {
		event := map[string]interface{}{
			"type": "location_update",
			"data": map[string]interface{}{
				"driver_id": *ride.DriverID,
				"lat":       loc.Lat,
				"lng":       loc.Lng,
				"heading":   loc.Heading,
				"speed":     loc.Speed,
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: location\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Keep connection open and send updates
	ctx := r.Context()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-clientChan:
			fmt.Fprintf(w, "event: location\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			// Send heartbeat
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"time\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()

			// Also send current location
			if loc, err := h.driverCache.GetDriverLocation(ctx, *ride.DriverID); err == nil && loc != nil {
				event := map[string]interface{}{
					"driver_id": *ride.DriverID,
					"lat":       loc.Lat,
					"lng":       loc.Lng,
					"heading":   loc.Heading,
					"speed":     loc.Speed,
					"timestamp": time.Now().Format(time.RFC3339),
				}
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: location\ndata: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}

func (h *SSEHandler) registerClient(rideID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[rideID] == nil {
		h.clients[rideID] = make(map[chan []byte]bool)
	}
	h.clients[rideID][ch] = true
}

func (h *SSEHandler) unregisterClient(rideID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[rideID]; ok {
		delete(clients, ch)
		if len(clients) == 0 {
			delete(h.clients, rideID)
		}
	}
	close(ch)
}

func (h *SSEHandler) BroadcastLocation(rideID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[rideID]; ok {
		for ch := range clients {
			select {
			case ch <- data:
			default:
				// Client too slow, skip
			}
		}
	}
}

// startPubSubListener listens for location updates via Redis pub/sub
func (h *SSEHandler) startPubSubListener() {
	ctx := context.Background()
	pubsub := h.redis.Subscribe(ctx, "driver:location:updates")
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		var update struct {
			RideID   string  `json:"ride_id"`
			DriverID string  `json:"driver_id"`
			Lat      float64 `json:"lat"`
			Lng      float64 `json:"lng"`
		}

		if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
			continue
		}

		event := map[string]interface{}{
			"driver_id": update.DriverID,
			"lat":       update.Lat,
			"lng":       update.Lng,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		data, _ := json.Marshal(event)

		h.BroadcastLocation(update.RideID, data)
	}
}

// PublishLocationUpdate publishes a location update to Redis
func PublishLocationUpdate(ctx context.Context, redis *redis.Client, rideID, driverID string, lat, lng float64) error {
	update := map[string]interface{}{
		"ride_id":   rideID,
		"driver_id": driverID,
		"lat":       lat,
		"lng":       lng,
	}
	data, _ := json.Marshal(update)
	return redis.Publish(ctx, "driver:location:updates", data).Err()
}

// NotificationHandler for sending notifications
type NotificationHandler struct {
	clients map[string]chan []byte // userID -> notification channel
	mu      sync.RWMutex
}

func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{
		clients: make(map[string]chan []byte),
	}
}

func (h *NotificationHandler) RegisterRoutes(r chi.Router) {
	r.Get("/users/{id}/notifications", h.StreamNotifications)
}

func (h *NotificationHandler) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "user id required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := make(chan []byte, 10)

	h.mu.Lock()
	h.clients[userID] = clientChan
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, userID)
		h.mu.Unlock()
		close(clientChan)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-clientChan:
			fmt.Fprintf(w, "event: notification\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func (h *NotificationHandler) SendNotification(userID string, notificationType string, data interface{}) {
	h.mu.RLock()
	ch, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		log.Printf("User %s not connected for notifications", userID)
		return
	}

	notification := map[string]interface{}{
		"type":      notificationType,
		"data":      data,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	msg, _ := json.Marshal(notification)
	select {
	case ch <- msg:
	default:
		log.Printf("Failed to send notification to user %s", userID)
	}
}
