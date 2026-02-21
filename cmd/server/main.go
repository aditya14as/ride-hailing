package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	"github.com/aditya/go-comet/internal/config"
	"github.com/aditya/go-comet/internal/database"
	"github.com/aditya/go-comet/internal/handler"
	"github.com/aditya/go-comet/internal/middleware"
	"github.com/aditya/go-comet/internal/repository"
	"github.com/aditya/go-comet/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize New Relic (optional)
	var nrApp *newrelic.Application
	if cfg.NewRelicEnabled && cfg.NewRelicLicenseKey != "" {
		nrApp, err = newrelic.NewApplication(
			newrelic.ConfigAppName(cfg.NewRelicAppName),
			newrelic.ConfigLicense(cfg.NewRelicLicenseKey),
			newrelic.ConfigDistributedTracerEnabled(true),
			newrelic.ConfigAppLogForwardingEnabled(true),
			newrelic.ConfigInfoLogger(os.Stdout),
		)
		if err != nil {
			log.Printf("Warning: Failed to initialize New Relic: %v", err)
		} else {
			log.Println("New Relic initialized successfully")
			// Wait for connection to establish
			if err := nrApp.WaitForConnection(10 * time.Second); err != nil {
				log.Printf("Warning: New Relic connection timeout: %v", err)
			} else {
				log.Println("New Relic connected successfully!")
			}
		}
	}

	// Initialize PostgreSQL
	db, err := database.NewPostgres(
		cfg.DatabaseURL,
		cfg.DBMaxConnections,
		cfg.DBMaxIdleConnections,
	)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Initialize Redis
	redis, err := database.NewRedis(cfg.RedisURL, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()
	log.Println("Connected to Redis")

	// Initialize cache
	driverCache := cache.NewDriverLocationCache(redis.Client)

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)
	driverRepo := repository.NewDriverRepository(db.DB)
	rideRepo := repository.NewRideRepository(db.DB)
	tripRepo := repository.NewTripRepository(db.DB)
	paymentRepo := repository.NewPaymentRepository(db.DB)
	offerRepo := repository.NewRideOfferRepository(db.DB)

	// Initialize services
	pricingService := service.NewPricingService()
	rideService := service.NewRideService(rideRepo, userRepo, driverRepo, pricingService, driverCache)
	driverService := service.NewDriverService(db.DB, driverRepo, rideRepo, tripRepo, offerRepo, userRepo, driverCache)
	tripService := service.NewTripService(tripRepo, rideRepo, driverRepo, pricingService, driverCache)
	paymentService := service.NewPaymentService(paymentRepo, tripRepo)
	matchingService := service.NewMatchingService(driverRepo, rideRepo, offerRepo, driverCache)

	// Initialize handlers
	userHandler := handler.NewUserHandler(userRepo)
	rideHandler := handler.NewRideHandler(rideService, matchingService)
	driverHandler := handler.NewDriverHandler(driverService, matchingService)
	tripHandler := handler.NewTripHandler(tripService)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	sseHandler := handler.NewSSEHandler(rideRepo, driverCache, redis.Client)

	// Create router
	r := chi.NewRouter()

	// Apply middleware
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// New Relic middleware
	if nrApp != nil {
		r.Use(middleware.NewRelicMiddleware(nrApp))
	}

	// Rate limiter (100 requests per minute per IP)
	rateLimiter := middleware.NewRateLimiter(redis.Client, 100, time.Minute)
	r.Use(rateLimiter.Handler)

	// Idempotency middleware
	idempotencyMw := middleware.NewIdempotencyMiddleware(redis.Client)
	r.Use(idempotencyMw.Handler)

	// Serve frontend
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend/index.html")
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check DB health
		if err := db.Health(ctx); err != nil {
			http.Error(w, "database unhealthy", http.StatusServiceUnavailable)
			return
		}

		// Check Redis health
		if err := redis.Health(ctx); err != nil {
			http.Error(w, "redis unhealthy", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","services":{"database":"up","redis":"up"}}`))
	})

	// API v1 routes
	r.Route("/v1", func(r chi.Router) {
		// Register all handlers
		userHandler.RegisterRoutes(r)
		rideHandler.RegisterRoutes(r)
		driverHandler.RegisterRoutes(r)
		tripHandler.RegisterRoutes(r)
		paymentHandler.RegisterRoutes(r)
		sseHandler.RegisterRoutes(r)
	})

	// Create server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start server
	log.Printf("Server starting on port %s", cfg.Port)
	log.Println("API endpoints:")
	log.Println("  POST /v1/users          - Create user")
	log.Println("  POST /v1/drivers        - Create driver")
	log.Println("  POST /v1/rides          - Create ride")
	log.Println("  GET  /v1/rides/{id}     - Get ride")
	log.Println("  POST /v1/drivers/{id}/location - Update location")
	log.Println("  POST /v1/drivers/{id}/accept   - Accept ride")
	log.Println("  POST /v1/trips/{id}/end        - End trip")
	log.Println("  POST /v1/payments              - Process payment")
	log.Println("  GET  /v1/rides/{id}/track      - SSE live tracking")
	log.Println("")
	log.Println("Frontend: http://localhost:" + cfg.Port)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped gracefully")
}
