//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	"github.com/aditya/go-comet/internal/config"
	"github.com/aditya/go-comet/internal/database"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
)

// Bangalore coordinates
const (
	baseLat = 12.9716
	baseLng = 77.5946
)

var (
	firstNames = []string{"Rahul", "Priya", "Amit", "Sneha", "Vikram", "Anita", "Raj", "Neha", "Suresh", "Kavita",
		"Arun", "Deepa", "Kiran", "Meera", "Sanjay", "Ritu", "Vijay", "Pooja", "Manoj", "Swati"}
	lastNames = []string{"Kumar", "Sharma", "Patel", "Singh", "Reddy", "Rao", "Gupta", "Joshi", "Nair", "Menon"}
)

func main() {
	rand.Seed(time.Now().UnixNano())

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.NewPostgres(cfg.DatabaseURL, cfg.DBMaxConnections, cfg.DBMaxIdleConnections)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	redis, err := database.NewRedis(cfg.RedisURL, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	ctx := context.Background()

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)
	driverRepo := repository.NewDriverRepository(db.DB)
	driverCache := cache.NewDriverLocationCache(redis.Client)

	// Create users
	log.Println("Creating 50 users...")
	userIDs := make([]string, 0)
	for i := 0; i < 50; i++ {
		user := &models.User{
			Phone:  fmt.Sprintf("98%08d", rand.Intn(100000000)),
			Name:   fmt.Sprintf("%s %s", firstNames[rand.Intn(len(firstNames))], lastNames[rand.Intn(len(lastNames))]),
			Rating: 4.0 + rand.Float64(),
		}

		if err := userRepo.Create(ctx, user); err != nil {
			log.Printf("Failed to create user: %v", err)
			continue
		}
		userIDs = append(userIDs, user.ID)
	}
	log.Printf("Created %d users", len(userIDs))

	// Create drivers
	vehicleTypes := []string{"auto", "mini", "sedan", "suv"}
	log.Println("Creating 100 drivers...")
	driverIDs := make([]string, 0)

	for i := 0; i < 100; i++ {
		vt := vehicleTypes[rand.Intn(len(vehicleTypes))]
		driver := &models.Driver{
			Phone:         fmt.Sprintf("91%08d", rand.Intn(100000000)),
			Name:          fmt.Sprintf("%s %s", firstNames[rand.Intn(len(firstNames))], lastNames[rand.Intn(len(lastNames))]),
			LicenseNumber: fmt.Sprintf("DL%07d", rand.Intn(10000000)),
			VehicleType:   vt,
			VehicleNumber: fmt.Sprintf("KA%02d%s%04d", rand.Intn(99), string(rune('A'+rand.Intn(26)))+string(rune('A'+rand.Intn(26))), rand.Intn(10000)),
			Rating:        4.0 + rand.Float64(),
		}

		if err := driverRepo.Create(ctx, driver); err != nil {
			log.Printf("Failed to create driver: %v", err)
			continue
		}
		driverIDs = append(driverIDs, driver.ID)

		// Set driver online and update location (50% chance)
		if rand.Float64() > 0.5 {
			lat := baseLat + (rand.Float64()-0.5)*0.1 // +/- 0.05 degrees (~5km)
			lng := baseLng + (rand.Float64()-0.5)*0.1

			driverRepo.UpdateStatus(ctx, driver.ID, models.DriverStatusOnline)
			driverRepo.UpdateLocation(ctx, driver.ID, lat, lng)

			// Update cache
			driverCache.SetDriverMeta(ctx, driver.ID, models.DriverStatusOnline, vt, driver.Rating)
			driverCache.UpdateLocation(ctx, driver.ID, lat, lng, nil, nil, nil)
		}
	}
	log.Printf("Created %d drivers", len(driverIDs))

	// Summary
	log.Println("\n=== Seed Data Summary ===")
	log.Printf("Users created: %d", len(userIDs))
	log.Printf("Drivers created: %d", len(driverIDs))
	log.Println("\nSample User ID:", userIDs[0])
	log.Println("Sample Driver ID:", driverIDs[0])
	log.Println("\nYou can now test with these IDs!")
}
