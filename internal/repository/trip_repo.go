package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type TripRepository interface {
	Create(ctx context.Context, trip *models.Trip) error
	GetByID(ctx context.Context, id string) (*models.Trip, error)
	GetByRideID(ctx context.Context, rideID string) (*models.Trip, error)
	Update(ctx context.Context, trip *models.Trip) error
	UpdateStatus(ctx context.Context, id, status string) error
	EndTrip(ctx context.Context, trip *models.Trip) error
	GetActiveTripByDriverID(ctx context.Context, driverID string) (*models.Trip, error)
}

type tripRepository struct {
	db *sqlx.DB
}

func NewTripRepository(db *sqlx.DB) TripRepository {
	return &tripRepository{db: db}
}

func (r *tripRepository) Create(ctx context.Context, trip *models.Trip) error {
	if trip.ID == "" {
		trip.ID = uuid.New().String()
	}
	now := time.Now()
	trip.CreatedAt = now
	trip.UpdatedAt = now
	trip.StartTime = &now
	trip.Status = models.TripStatusStarted

	query := `
		INSERT INTO trips (id, ride_id, driver_id, user_id, status, start_time,
			pause_duration_secs, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		trip.ID, trip.RideID, trip.DriverID, trip.UserID, trip.Status,
		trip.StartTime, 0, trip.CreatedAt, trip.UpdatedAt)
	return err
}

func (r *tripRepository) GetByID(ctx context.Context, id string) (*models.Trip, error) {
	var trip models.Trip
	query := `SELECT * FROM trips WHERE id = $1`
	err := r.db.GetContext(ctx, &trip, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &trip, err
}

func (r *tripRepository) GetByRideID(ctx context.Context, rideID string) (*models.Trip, error) {
	var trip models.Trip
	query := `SELECT * FROM trips WHERE ride_id = $1`
	err := r.db.GetContext(ctx, &trip, query, rideID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &trip, err
}

func (r *tripRepository) Update(ctx context.Context, trip *models.Trip) error {
	trip.UpdatedAt = time.Now()
	query := `
		UPDATE trips
		SET status = $1, pause_duration_secs = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query,
		trip.Status, trip.PauseDurationSecs, trip.UpdatedAt, trip.ID)
	return err
}

func (r *tripRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE trips SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

func (r *tripRepository) EndTrip(ctx context.Context, trip *models.Trip) error {
	now := time.Now()
	trip.EndTime = &now
	trip.UpdatedAt = now
	trip.Status = models.TripStatusCompleted

	query := `
		UPDATE trips
		SET status = $1, end_time = $2, actual_distance_km = $3, actual_duration_mins = $4,
			base_fare = $5, distance_fare = $6, time_fare = $7, surge_amount = $8,
			total_fare = $9, updated_at = $10
		WHERE id = $11
	`
	_, err := r.db.ExecContext(ctx, query,
		trip.Status, trip.EndTime, trip.ActualDistanceKm, trip.ActualDurationMin,
		trip.BaseFare, trip.DistanceFare, trip.TimeFare, trip.SurgeAmount,
		trip.TotalFare, trip.UpdatedAt, trip.ID)
	return err
}

func (r *tripRepository) GetActiveTripByDriverID(ctx context.Context, driverID string) (*models.Trip, error) {
	var trip models.Trip
	query := `
		SELECT * FROM trips
		WHERE driver_id = $1 AND status IN ($2, $3)
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &trip, query, driverID, models.TripStatusStarted, models.TripStatusPaused)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &trip, err
}
