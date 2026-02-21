package service

import (
	"context"
	"log"

	"github.com/aditya/go-comet/internal/cache"
	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
)

type RideService interface {
	CreateRide(ctx context.Context, req *models.CreateRideRequest, idempotencyKey string) (*models.Ride, error)
	GetRide(ctx context.Context, id string) (*models.RideResponse, error)
	CancelRide(ctx context.Context, id string, req *models.CancelRideRequest) error
	UpdateRideStatus(ctx context.Context, id, status string) error
}

type rideService struct {
	rideRepo       repository.RideRepository
	userRepo       repository.UserRepository
	driverRepo     repository.DriverRepository
	pricingService PricingService
	driverCache    cache.DriverLocationCache
}

func NewRideService(
	rideRepo repository.RideRepository,
	userRepo repository.UserRepository,
	driverRepo repository.DriverRepository,
	pricingService PricingService,
	driverCache cache.DriverLocationCache,
) RideService {
	return &rideService{
		rideRepo:       rideRepo,
		userRepo:       userRepo,
		driverRepo:     driverRepo,
		pricingService: pricingService,
		driverCache:    driverCache,
	}
}

func (s *rideService) CreateRide(ctx context.Context, req *models.CreateRideRequest, idempotencyKey string) (*models.Ride, error) {
	// Check idempotency
	if idempotencyKey != "" {
		existingRide, err := s.rideRepo.GetByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existingRide != nil {
			return existingRide, nil
		}
	}

	// Check if user exists
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperrors.NotFound("user")
	}

	// Check if user has active ride
	activeRide, err := s.rideRepo.GetActiveRideByUserID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if activeRide != nil {
		return nil, apperrors.UserHasActiveRide()
	}

	// Calculate estimated distance and duration
	distanceKm := s.pricingService.EstimateDistance(
		req.Pickup.Lat, req.Pickup.Lng,
		req.Dropoff.Lat, req.Dropoff.Lng,
	)
	durationMins := s.pricingService.EstimateDuration(distanceKm)

	// Calculate surge based on demand/supply
	surgeMultiplier := 1.0
	if s.driverCache != nil {
		nearbyDrivers, _ := s.driverCache.GetNearbyDrivers(ctx, req.Pickup.Lat, req.Pickup.Lng, 2.0, req.VehicleType)
		// Simple surge: if less than 5 drivers nearby, apply surge
		if len(nearbyDrivers) < 5 {
			surgeMultiplier = s.pricingService.CalculateSurge(10, len(nearbyDrivers))
		}
	}

	// Calculate fare
	fare := s.pricingService.CalculateEstimatedFare(req.VehicleType, distanceKm, durationMins, surgeMultiplier)

	// Create ride
	ride := &models.Ride{
		UserID:        req.UserID,
		PickupLat:     req.Pickup.Lat,
		PickupLng:     req.Pickup.Lng,
		DropoffLat:    req.Dropoff.Lat,
		DropoffLng:    req.Dropoff.Lng,
		VehicleType:   req.VehicleType,
		PaymentMethod: req.PaymentMethod,
		Status:        models.RideStatusPending,
	}

	if req.Pickup.Address != "" {
		ride.PickupAddress = &req.Pickup.Address
	}
	if req.Dropoff.Address != "" {
		ride.DropoffAddress = &req.Dropoff.Address
	}
	if idempotencyKey != "" {
		ride.IdempotencyKey = &idempotencyKey
	}

	ride.EstimatedFare = &fare.Total
	ride.SurgeMultiplier = surgeMultiplier
	ride.EstimatedDistanceKm = &distanceKm
	ride.EstimatedDurationMin = &durationMins

	if err := s.rideRepo.Create(ctx, ride); err != nil {
		return nil, err
	}

	// Update status to matching
	if err := s.rideRepo.UpdateStatus(ctx, ride.ID, models.RideStatusMatching); err != nil {
		log.Printf("failed to update ride status to matching: %v", err)
	}
	ride.Status = models.RideStatusMatching

	return ride, nil
}

func (s *rideService) GetRide(ctx context.Context, id string) (*models.RideResponse, error) {
	ride, err := s.rideRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ride == nil {
		return nil, apperrors.NotFound("ride")
	}

	response := ride.ToResponse()

	// Fetch user
	user, err := s.userRepo.GetByID(ctx, ride.UserID)
	if err == nil && user != nil {
		response.User = user.ToResponse()
	}

	// Fetch driver if assigned
	if ride.DriverID != nil {
		driver, err := s.driverRepo.GetByID(ctx, *ride.DriverID)
		if err == nil && driver != nil {
			response.Driver = driver.ToResponse()

			// Get driver's current location from cache
			if s.driverCache != nil {
				loc, err := s.driverCache.GetDriverLocation(ctx, driver.ID)
				if err == nil && loc != nil {
					response.Driver.CurrentLat = &loc.Lat
					response.Driver.CurrentLng = &loc.Lng
				}
			}
		}
	}

	return response, nil
}

func (s *rideService) CancelRide(ctx context.Context, id string, req *models.CancelRideRequest) error {
	ride, err := s.rideRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ride == nil {
		return apperrors.NotFound("ride")
	}

	if !ride.CanTransitionTo(models.RideStatusCancelled) {
		return apperrors.InvalidTransition(ride.Status, models.RideStatusCancelled)
	}

	if err := s.rideRepo.Cancel(ctx, id, req.CancelledBy, req.Reason); err != nil {
		return err
	}

	// If driver was assigned, make them available again
	if ride.DriverID != nil {
		if err := s.driverRepo.UpdateStatus(ctx, *ride.DriverID, models.DriverStatusOnline); err != nil {
			log.Printf("failed to update driver status after cancellation: %v", err)
		}
	}

	return nil
}

func (s *rideService) UpdateRideStatus(ctx context.Context, id, status string) error {
	ride, err := s.rideRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if ride == nil {
		return apperrors.NotFound("ride")
	}

	if !ride.CanTransitionTo(status) {
		return apperrors.InvalidTransition(ride.Status, status)
	}

	return s.rideRepo.UpdateStatus(ctx, id, status)
}
