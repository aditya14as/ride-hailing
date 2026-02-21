package service

import (
	"context"
	"log"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
)

type TripService interface {
	StartTrip(ctx context.Context, rideID string) (*models.Trip, error)
	EndTrip(ctx context.Context, tripID string, req *models.EndTripRequest) (*models.TripResponse, error)
	GetTrip(ctx context.Context, tripID string) (*models.Trip, error)
	PauseTrip(ctx context.Context, tripID string) error
	ResumeTrip(ctx context.Context, tripID string) error
}

type tripService struct {
	tripRepo       repository.TripRepository
	rideRepo       repository.RideRepository
	driverRepo     repository.DriverRepository
	pricingService PricingService
	driverCache    cache.DriverLocationCache
}

func NewTripService(
	tripRepo repository.TripRepository,
	rideRepo repository.RideRepository,
	driverRepo repository.DriverRepository,
	pricingService PricingService,
	driverCache cache.DriverLocationCache,
) TripService {
	return &tripService{
		tripRepo:       tripRepo,
		rideRepo:       rideRepo,
		driverRepo:     driverRepo,
		pricingService: pricingService,
		driverCache:    driverCache,
	}
}

func (s *tripService) StartTrip(ctx context.Context, rideID string) (*models.Trip, error) {
	ride, err := s.rideRepo.GetByID(ctx, rideID)
	if err != nil {
		return nil, err
	}
	if ride == nil {
		return nil, apperrors.NotFound("ride")
	}

	// Check if ride can start
	if ride.Status != models.RideStatusDriverArrived {
		return nil, apperrors.BadRequest("driver must arrive before starting trip")
	}

	if ride.DriverID == nil {
		return nil, apperrors.BadRequest("no driver assigned")
	}

	// Check if trip already exists
	existingTrip, err := s.tripRepo.GetByRideID(ctx, rideID)
	if err != nil {
		return nil, err
	}
	if existingTrip != nil {
		return existingTrip, nil
	}

	// Create trip
	trip := &models.Trip{
		RideID:   rideID,
		DriverID: *ride.DriverID,
		UserID:   ride.UserID,
		Status:   models.TripStatusStarted,
	}

	if err := s.tripRepo.Create(ctx, trip); err != nil {
		return nil, err
	}

	// Update ride status
	if err := s.rideRepo.UpdateStatus(ctx, rideID, models.RideStatusInProgress); err != nil {
		log.Printf("failed to update ride status: %v", err)
	}

	return trip, nil
}

func (s *tripService) EndTrip(ctx context.Context, tripID string, req *models.EndTripRequest) (*models.TripResponse, error) {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip == nil {
		return nil, apperrors.NotFound("trip")
	}

	if !trip.CanTransitionTo(models.TripStatusCompleted) {
		return nil, apperrors.InvalidTransition(trip.Status, models.TripStatusCompleted)
	}

	// Get ride for surge multiplier and vehicle type
	ride, err := s.rideRepo.GetByID(ctx, trip.RideID)
	if err != nil {
		return nil, err
	}
	if ride == nil {
		return nil, apperrors.NotFound("ride")
	}

	// Calculate actual distance and duration
	var actualDistanceKm float64
	if req.OdometerKm != nil {
		actualDistanceKm = *req.OdometerKm
	} else if ride.EstimatedDistanceKm != nil {
		actualDistanceKm = *ride.EstimatedDistanceKm
	} else {
		// Calculate from coordinates
		actualDistanceKm = s.pricingService.EstimateDistance(
			ride.PickupLat, ride.PickupLng,
			req.EndLat, req.EndLng,
		)
	}

	// Calculate duration
	var actualDurationMins int
	if trip.StartTime != nil {
		duration := time.Since(*trip.StartTime)
		actualDurationMins = int(duration.Minutes()) - (trip.PauseDurationSecs / 60)
		if actualDurationMins < 1 {
			actualDurationMins = 1
		}
	} else {
		actualDurationMins = s.pricingService.EstimateDuration(actualDistanceKm)
	}

	// Calculate fare
	fare := s.pricingService.CalculateActualFare(
		ride.VehicleType,
		actualDistanceKm,
		actualDurationMins,
		ride.SurgeMultiplier,
	)

	// Update trip
	trip.ActualDistanceKm = &actualDistanceKm
	trip.ActualDurationMin = &actualDurationMins
	trip.BaseFare = &fare.BaseFare
	trip.DistanceFare = &fare.DistanceFare
	trip.TimeFare = &fare.TimeFare
	trip.SurgeAmount = &fare.SurgeAmount
	trip.TotalFare = &fare.Total
	trip.Status = models.TripStatusCompleted

	if err := s.tripRepo.EndTrip(ctx, trip); err != nil {
		return nil, err
	}

	// Update ride status
	if err := s.rideRepo.UpdateStatus(ctx, trip.RideID, models.RideStatusCompleted); err != nil {
		log.Printf("failed to update ride status: %v", err)
	}

	// Update driver status and stats
	if err := s.driverRepo.UpdateStatus(ctx, trip.DriverID, models.DriverStatusOnline); err != nil {
		log.Printf("failed to update driver status: %v", err)
	}
	if err := s.driverRepo.IncrementTotalTrips(ctx, trip.DriverID); err != nil {
		log.Printf("failed to increment driver trips: %v", err)
	}

	// Clear cache
	if s.driverCache != nil {
		s.driverCache.ClearActiveRide(ctx, trip.DriverID)
		s.driverCache.ClearUserActiveRide(ctx, trip.UserID)
	}

	return trip.ToResponse(), nil
}

func (s *tripService) GetTrip(ctx context.Context, tripID string) (*models.Trip, error) {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip == nil {
		return nil, apperrors.NotFound("trip")
	}
	return trip, nil
}

func (s *tripService) PauseTrip(ctx context.Context, tripID string) error {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return err
	}
	if trip == nil {
		return apperrors.NotFound("trip")
	}

	if !trip.CanTransitionTo(models.TripStatusPaused) {
		return apperrors.InvalidTransition(trip.Status, models.TripStatusPaused)
	}

	return s.tripRepo.UpdateStatus(ctx, tripID, models.TripStatusPaused)
}

func (s *tripService) ResumeTrip(ctx context.Context, tripID string) error {
	trip, err := s.tripRepo.GetByID(ctx, tripID)
	if err != nil {
		return err
	}
	if trip == nil {
		return apperrors.NotFound("trip")
	}

	if trip.Status != models.TripStatusPaused {
		return apperrors.BadRequest("trip is not paused")
	}

	return s.tripRepo.UpdateStatus(ctx, tripID, models.TripStatusStarted)
}
