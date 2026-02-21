package service

import (
	"context"
	"log"
	"time"

	"github.com/aditya/go-comet/internal/cache"
	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
	"github.com/jmoiron/sqlx"
)

type DriverService interface {
	CreateDriver(ctx context.Context, req *models.CreateDriverRequest) (*models.Driver, error)
	GetDriver(ctx context.Context, id string) (*models.Driver, error)
	UpdateLocation(ctx context.Context, driverID string, req *models.UpdateDriverLocationRequest) error
	GoOnline(ctx context.Context, driverID string) error
	GoOffline(ctx context.Context, driverID string) error
	AcceptRide(ctx context.Context, driverID string, req *models.AcceptRideRequest) (*models.RideResponse, error)
	DeclineRide(ctx context.Context, driverID, offerID string) error
}

type driverService struct {
	db            *sqlx.DB
	driverRepo    repository.DriverRepository
	rideRepo      repository.RideRepository
	tripRepo      repository.TripRepository
	offerRepo     repository.RideOfferRepository
	userRepo      repository.UserRepository
	driverCache   cache.DriverLocationCache
}

func NewDriverService(
	db *sqlx.DB,
	driverRepo repository.DriverRepository,
	rideRepo repository.RideRepository,
	tripRepo repository.TripRepository,
	offerRepo repository.RideOfferRepository,
	userRepo repository.UserRepository,
	driverCache cache.DriverLocationCache,
) DriverService {
	return &driverService{
		db:            db,
		driverRepo:    driverRepo,
		rideRepo:      rideRepo,
		tripRepo:      tripRepo,
		offerRepo:     offerRepo,
		userRepo:      userRepo,
		driverCache:   driverCache,
	}
}

func (s *driverService) CreateDriver(ctx context.Context, req *models.CreateDriverRequest) (*models.Driver, error) {
	// Check if phone already exists
	existing, err := s.driverRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperrors.Conflict("driver with this phone already exists")
	}

	driver := &models.Driver{
		Phone:         req.Phone,
		Name:          req.Name,
		LicenseNumber: req.LicenseNumber,
		VehicleType:   req.VehicleType,
		VehicleNumber: req.VehicleNumber,
	}

	if req.Email != "" {
		driver.Email = &req.Email
	}

	if err := s.driverRepo.Create(ctx, driver); err != nil {
		return nil, err
	}

	return driver, nil
}

func (s *driverService) GetDriver(ctx context.Context, id string) (*models.Driver, error) {
	driver, err := s.driverRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, apperrors.NotFound("driver")
	}

	// Get current location from cache
	if s.driverCache != nil {
		loc, err := s.driverCache.GetDriverLocation(ctx, id)
		if err == nil && loc != nil {
			driver.CurrentLat = &loc.Lat
			driver.CurrentLng = &loc.Lng
		}
	}

	return driver, nil
}

func (s *driverService) UpdateLocation(ctx context.Context, driverID string, req *models.UpdateDriverLocationRequest) error {
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err != nil {
		return err
	}
	if driver == nil {
		return apperrors.NotFound("driver")
	}

	if driver.Status == models.DriverStatusOffline {
		return apperrors.BadRequest("driver is offline")
	}

	// Update cache (primary - fast)
	if s.driverCache != nil {
		if err := s.driverCache.UpdateLocation(ctx, driverID, req.Lat, req.Lng, req.Heading, req.Speed, req.Accuracy); err != nil {
			log.Printf("failed to update driver location in cache: %v", err)
		}
	}

	// Update database (secondary - for persistence)
	if err := s.driverRepo.UpdateLocation(ctx, driverID, req.Lat, req.Lng); err != nil {
		log.Printf("failed to update driver location in db: %v", err)
	}

	return nil
}

func (s *driverService) GoOnline(ctx context.Context, driverID string) error {
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err != nil {
		return err
	}
	if driver == nil {
		return apperrors.NotFound("driver")
	}

	if err := s.driverRepo.UpdateStatus(ctx, driverID, models.DriverStatusOnline); err != nil {
		return err
	}

	// Update cache
	if s.driverCache != nil {
		if err := s.driverCache.SetDriverMeta(ctx, driverID, models.DriverStatusOnline, driver.VehicleType, driver.Rating); err != nil {
			log.Printf("failed to set driver meta in cache: %v", err)
		}
	}

	return nil
}

func (s *driverService) GoOffline(ctx context.Context, driverID string) error {
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err != nil {
		return err
	}
	if driver == nil {
		return apperrors.NotFound("driver")
	}

	// Check if driver has active ride
	activeRide, err := s.rideRepo.GetActiveRideByDriverID(ctx, driverID)
	if err != nil {
		return err
	}
	if activeRide != nil {
		return apperrors.BadRequest("cannot go offline with active ride")
	}

	if err := s.driverRepo.UpdateStatus(ctx, driverID, models.DriverStatusOffline); err != nil {
		return err
	}

	// Update cache
	if s.driverCache != nil {
		s.driverCache.SetDriverMeta(ctx, driverID, models.DriverStatusOffline, driver.VehicleType, driver.Rating)
		s.driverCache.RemoveDriver(ctx, driverID, driver.VehicleType)
	}

	return nil
}

func (s *driverService) AcceptRide(ctx context.Context, driverID string, req *models.AcceptRideRequest) (*models.RideResponse, error) {
	// Use transaction for atomicity
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get offer with lock
	offer, err := s.offerRepo.GetByIDForUpdate(ctx, tx, req.OfferID)
	if err != nil {
		return nil, err
	}
	if offer == nil {
		return nil, apperrors.NotFound("offer")
	}

	// Validate offer
	if offer.DriverID != driverID {
		return nil, apperrors.Unauthorized("offer not for this driver")
	}
	if offer.RideID != req.RideID {
		return nil, apperrors.BadRequest("offer ride mismatch")
	}
	if offer.IsExpired() {
		return nil, apperrors.OfferExpired()
	}
	if offer.Status != models.OfferStatusPending {
		return nil, apperrors.BadRequest("offer already responded")
	}

	// Get ride with lock
	ride, err := s.rideRepo.GetByIDForUpdate(ctx, tx, req.RideID)
	if err != nil {
		return nil, err
	}
	if ride == nil {
		return nil, apperrors.NotFound("ride")
	}

	// Check if ride is still available
	if ride.Status != models.RideStatusMatching {
		return nil, apperrors.RideAlreadyAssigned()
	}

	// Update offer status
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		"UPDATE ride_offers SET status = $1, responded_at = $2 WHERE id = $3",
		models.OfferStatusAccepted, now, offer.ID)
	if err != nil {
		return nil, err
	}

	// Assign driver to ride
	_, err = tx.ExecContext(ctx,
		"UPDATE rides SET driver_id = $1, status = $2, updated_at = $3 WHERE id = $4",
		driverID, models.RideStatusDriverAssigned, now, ride.ID)
	if err != nil {
		return nil, err
	}

	// Update driver status to busy
	_, err = tx.ExecContext(ctx,
		"UPDATE drivers SET status = $1, updated_at = $2 WHERE id = $3",
		models.DriverStatusBusy, now, driverID)
	if err != nil {
		return nil, err
	}

	// Expire other pending offers for this ride
	_, err = tx.ExecContext(ctx,
		"UPDATE ride_offers SET status = $1, responded_at = $2 WHERE ride_id = $3 AND status = $4",
		models.OfferStatusExpired, now, ride.ID, models.OfferStatusPending)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Update cache
	if s.driverCache != nil {
		s.driverCache.SetActiveRide(ctx, driverID, ride.ID)
	}

	// Get updated ride with user info
	ride.DriverID = &driverID
	ride.Status = models.RideStatusDriverAssigned

	response := ride.ToResponse()

	// Fetch user
	user, err := s.userRepo.GetByID(ctx, ride.UserID)
	if err == nil && user != nil {
		response.User = user.ToResponse()
	}

	// Fetch driver
	driver, err := s.driverRepo.GetByID(ctx, driverID)
	if err == nil && driver != nil {
		response.Driver = driver.ToResponse()
	}

	return response, nil
}

func (s *driverService) DeclineRide(ctx context.Context, driverID, offerID string) error {
	offer, err := s.offerRepo.GetByID(ctx, offerID)
	if err != nil {
		return err
	}
	if offer == nil {
		return apperrors.NotFound("offer")
	}

	if offer.DriverID != driverID {
		return apperrors.Unauthorized("offer not for this driver")
	}

	if offer.Status != models.OfferStatusPending {
		return apperrors.BadRequest("offer already responded")
	}

	return s.offerRepo.UpdateStatus(ctx, offerID, models.OfferStatusDeclined)
}
