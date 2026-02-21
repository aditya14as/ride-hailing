package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port string
	Env  string

	// Database
	DatabaseURL          string
	DBMaxConnections     int
	DBMaxIdleConnections int

	// Redis
	RedisURL      string
	RedisPassword string

	// New Relic
	NewRelicLicenseKey string
	NewRelicAppName    string
	NewRelicEnabled    bool

	// Matching
	MatchingRadiusKM    float64
	OfferTimeoutSeconds int
	MaxMatchingRetries  int
}

func Load() (*Config, error) {
	// Load .env file if exists
	godotenv.Load()

	return &Config{
		// Server
		Port: getEnv("PORT", "8080"),
		Env:  getEnv("ENV", "development"),

		// Database
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://gocomet:gocomet123@localhost:5432/gocomet?sslmode=disable"),
		DBMaxConnections:     getEnvAsInt("DB_MAX_CONNECTIONS", 25),
		DBMaxIdleConnections: getEnvAsInt("DB_MAX_IDLE_CONNECTIONS", 5),

		// Redis
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// New Relic
		NewRelicLicenseKey: getEnv("NEW_RELIC_LICENSE_KEY", ""),
		NewRelicAppName:    getEnv("NEW_RELIC_APP_NAME", "gocomet-ride-hailing"),
		NewRelicEnabled:    getEnvAsBool("NEW_RELIC_ENABLED", false),

		// Matching
		MatchingRadiusKM:    getEnvAsFloat("MATCHING_RADIUS_KM", 5.0),
		OfferTimeoutSeconds: getEnvAsInt("OFFER_TIMEOUT_SECONDS", 15),
		MaxMatchingRetries:  getEnvAsInt("MAX_MATCHING_RETRIES", 3),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
