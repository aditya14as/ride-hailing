package service

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
)

const (
	defaultOfferTimeout = 15 * time.Second
	defaultMatchRadius  = 5.0 // km
	maxRetries          = 3
)

type MatchingService interface {
	FindAndOfferDrivers(ctx context.Context, ride *models.Ride) error
	GetPendingOffers(ctx context.Context, driverID string) ([]*models.RideOfferResponse, error)
}

type ScoredDriver struct {
	DriverID string
	Score    float64
	Distance float64
}

type matchingService struct {
	driverRepo    repository.DriverRepository
	rideRepo      repository.RideRepository
	offerRepo     repository.RideOfferRepository
	driverCache   cache.DriverLocationCache
	offerTimeout  time.Duration
	matchRadius   float64
}

func NewMatchingService(
	driverRepo repository.DriverRepository,
	rideRepo repository.RideRepository,
	offerRepo repository.RideOfferRepository,
	driverCache cache.DriverLocationCache,
) MatchingService {
	return &matchingService{
		driverRepo:   driverRepo,
		rideRepo:     rideRepo,
		offerRepo:    offerRepo,
		driverCache:  driverCache,
		offerTimeout: defaultOfferTimeout,
		matchRadius:  defaultMatchRadius,
	}
}

func (s *matchingService) FindAndOfferDrivers(ctx context.Context, ride *models.Ride) error {
	// Get nearby drivers from cache
	nearbyDrivers, err := s.driverCache.GetNearbyDrivers(
		ctx,
		ride.PickupLat,
		ride.PickupLng,
		s.matchRadius,
		ride.VehicleType,
	)
	if err != nil {
		log.Printf("error getting nearby drivers: %v", err)
		return err
	}

	if len(nearbyDrivers) == 0 {
		// Try database fallback
		dbDrivers, err := s.driverRepo.GetOnlineDriversByVehicleType(ctx, ride.VehicleType)
		if err != nil {
			return err
		}

		if len(dbDrivers) == 0 {
			// Cancel ride - no drivers
			if err := s.rideRepo.Cancel(ctx, ride.ID, "system", "no drivers available"); err != nil {
				log.Printf("failed to cancel ride: %v", err)
			}
			return apperrors.ErrNoDriversAvailable
		}

		// Convert to cache format
		for _, d := range dbDrivers {
			if d.CurrentLat != nil && d.CurrentLng != nil {
				nearbyDrivers = append(nearbyDrivers, cache.DriverWithDistance{
					DriverID: d.ID,
					Distance: 0, // Will be calculated
				})
			}
		}
	}

	// Score and sort drivers
	scoredDrivers := s.scoreDrivers(ctx, nearbyDrivers, ride)
	if len(scoredDrivers) == 0 {
		return apperrors.ErrNoDriversAvailable
	}

	// Create offers for top drivers (up to 3)
	maxOffers := 3
	if len(scoredDrivers) < maxOffers {
		maxOffers = len(scoredDrivers)
	}

	for i := 0; i < maxOffers; i++ {
		driver := scoredDrivers[i]
		offer := &models.RideOffer{
			RideID:    ride.ID,
			DriverID:  driver.DriverID,
			ExpiresAt: time.Now().Add(s.offerTimeout),
		}

		if err := s.offerRepo.Create(ctx, offer); err != nil {
			log.Printf("failed to create offer for driver %s: %v", driver.DriverID, err)
			continue
		}

		log.Printf("created offer %s for driver %s (score: %.2f, distance: %.2f km)",
			offer.ID, driver.DriverID, driver.Score, driver.Distance)
	}

	return nil
}

func (s *matchingService) scoreDrivers(ctx context.Context, drivers []cache.DriverWithDistance, ride *models.Ride) []ScoredDriver {
	scored := make([]ScoredDriver, 0, len(drivers))

	for _, d := range drivers {
		// Skip if driver already has pending offer for this ride
		existing, _ := s.offerRepo.GetByRideAndDriver(ctx, ride.ID, d.DriverID)
		if existing != nil {
			continue
		}

		// Get driver metadata from cache
		meta, err := s.driverCache.GetDriverMeta(ctx, d.DriverID)
		if err != nil {
			continue
		}

		// Skip if not online
		if meta["status"] != models.DriverStatusOnline {
			continue
		}

		// Check if driver has active ride
		activeRide, _ := s.driverCache.GetActiveRide(ctx, d.DriverID)
		if activeRide != "" {
			continue
		}

		// Calculate score
		score := 100.0

		// Distance penalty (closer = better)
		score -= d.Distance * 10 // -10 points per km

		// Rating bonus
		rating := cache.ParseRating(meta["rating"])
		score += rating * 5 // +25 points for 5-star

		scored = append(scored, ScoredDriver{
			DriverID: d.DriverID,
			Score:    score,
			Distance: d.Distance,
		})
	}

	// Sort by score (highest first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return scored
}

func (s *matchingService) GetPendingOffers(ctx context.Context, driverID string) ([]*models.RideOfferResponse, error) {
	offers, err := s.offerRepo.GetPendingByDriverID(ctx, driverID)
	if err != nil {
		return nil, err
	}

	responses := make([]*models.RideOfferResponse, 0, len(offers))
	for _, offer := range offers {
		response := offer.ToResponse()

		// Get ride details
		ride, err := s.rideRepo.GetByID(ctx, offer.RideID)
		if err == nil && ride != nil {
			response.Ride = ride.ToResponse()
		}

		responses = append(responses, response)
	}

	return responses, nil
}
