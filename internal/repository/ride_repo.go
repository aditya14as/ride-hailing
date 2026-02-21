package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RideRepository interface {
	Create(ctx context.Context, ride *models.Ride) error
	GetByID(ctx context.Context, id string) (*models.Ride, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Ride, error)
	Update(ctx context.Context, ride *models.Ride) error
	UpdateStatus(ctx context.Context, id, status string) error
	AssignDriver(ctx context.Context, rideID, driverID string) error
	Cancel(ctx context.Context, id, cancelledBy, reason string) error
	GetActiveRideByUserID(ctx context.Context, userID string) (*models.Ride, error)
	GetActiveRideByDriverID(ctx context.Context, driverID string) (*models.Ride, error)
	GetByIDForUpdate(ctx context.Context, tx *sqlx.Tx, id string) (*models.Ride, error)
}

type rideRepository struct {
	db *sqlx.DB
}

func NewRideRepository(db *sqlx.DB) RideRepository {
	return &rideRepository{db: db}
}

func (r *rideRepository) Create(ctx context.Context, ride *models.Ride) error {
	if ride.ID == "" {
		ride.ID = uuid.New().String()
	}
	ride.CreatedAt = time.Now()
	ride.UpdatedAt = time.Now()
	ride.Status = models.RideStatusPending
	ride.SurgeMultiplier = 1.0

	query := `
		INSERT INTO rides (id, user_id, pickup_lat, pickup_lng, pickup_address,
			dropoff_lat, dropoff_lng, dropoff_address, vehicle_type, status,
			estimated_fare, surge_multiplier, estimated_distance_km, estimated_duration_mins,
			payment_method, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	_, err := r.db.ExecContext(ctx, query,
		ride.ID, ride.UserID, ride.PickupLat, ride.PickupLng, ride.PickupAddress,
		ride.DropoffLat, ride.DropoffLng, ride.DropoffAddress, ride.VehicleType, ride.Status,
		ride.EstimatedFare, ride.SurgeMultiplier, ride.EstimatedDistanceKm, ride.EstimatedDurationMin,
		ride.PaymentMethod, ride.IdempotencyKey, ride.CreatedAt, ride.UpdatedAt)
	return err
}

func (r *rideRepository) GetByID(ctx context.Context, id string) (*models.Ride, error) {
	var ride models.Ride
	query := `SELECT * FROM rides WHERE id = $1`
	err := r.db.GetContext(ctx, &ride, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ride, err
}

func (r *rideRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Ride, error) {
	var ride models.Ride
	query := `SELECT * FROM rides WHERE idempotency_key = $1`
	err := r.db.GetContext(ctx, &ride, query, key)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ride, err
}

func (r *rideRepository) Update(ctx context.Context, ride *models.Ride) error {
	ride.UpdatedAt = time.Now()
	query := `
		UPDATE rides
		SET status = $1, driver_id = $2, estimated_fare = $3, surge_multiplier = $4,
			estimated_distance_km = $5, estimated_duration_mins = $6, updated_at = $7
		WHERE id = $8
	`
	_, err := r.db.ExecContext(ctx, query,
		ride.Status, ride.DriverID, ride.EstimatedFare, ride.SurgeMultiplier,
		ride.EstimatedDistanceKm, ride.EstimatedDurationMin, ride.UpdatedAt, ride.ID)
	return err
}

func (r *rideRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE rides SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

func (r *rideRepository) AssignDriver(ctx context.Context, rideID, driverID string) error {
	query := `UPDATE rides SET driver_id = $1, status = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, driverID, models.RideStatusDriverAssigned, time.Now(), rideID)
	return err
}

func (r *rideRepository) Cancel(ctx context.Context, id, cancelledBy, reason string) error {
	query := `
		UPDATE rides
		SET status = $1, cancelled_by = $2, cancellation_reason = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		models.RideStatusCancelled, cancelledBy, reason, time.Now(), id)
	return err
}

func (r *rideRepository) GetActiveRideByUserID(ctx context.Context, userID string) (*models.Ride, error) {
	var ride models.Ride
	query := `
		SELECT * FROM rides
		WHERE user_id = $1 AND status NOT IN ($2, $3)
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &ride, query, userID, models.RideStatusCompleted, models.RideStatusCancelled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ride, err
}

func (r *rideRepository) GetActiveRideByDriverID(ctx context.Context, driverID string) (*models.Ride, error) {
	var ride models.Ride
	query := `
		SELECT * FROM rides
		WHERE driver_id = $1 AND status NOT IN ($2, $3)
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &ride, query, driverID, models.RideStatusCompleted, models.RideStatusCancelled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ride, err
}

// GetByIDForUpdate gets a ride with a FOR UPDATE lock (for preventing race conditions)
func (r *rideRepository) GetByIDForUpdate(ctx context.Context, tx *sqlx.Tx, id string) (*models.Ride, error) {
	var ride models.Ride
	query := `SELECT * FROM rides WHERE id = $1 FOR UPDATE`
	err := tx.GetContext(ctx, &ride, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ride, err
}
