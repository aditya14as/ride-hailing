package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type DriverRepository interface {
	Create(ctx context.Context, driver *models.Driver) error
	GetByID(ctx context.Context, id string) (*models.Driver, error)
	GetByPhone(ctx context.Context, phone string) (*models.Driver, error)
	Update(ctx context.Context, driver *models.Driver) error
	UpdateStatus(ctx context.Context, id string, status string) error
	UpdateLocation(ctx context.Context, id string, lat, lng float64) error
	UpdateRating(ctx context.Context, id string, rating float64) error
	IncrementTotalTrips(ctx context.Context, id string) error
	GetOnlineDriversByVehicleType(ctx context.Context, vehicleType string) ([]*models.Driver, error)
}

type driverRepository struct {
	db *sqlx.DB
}

func NewDriverRepository(db *sqlx.DB) DriverRepository {
	return &driverRepository{db: db}
}

func (r *driverRepository) Create(ctx context.Context, driver *models.Driver) error {
	if driver.ID == "" {
		driver.ID = uuid.New().String()
	}
	driver.CreatedAt = time.Now()
	driver.UpdatedAt = time.Now()
	driver.Rating = 5.0
	driver.TotalTrips = 0
	driver.Status = models.DriverStatusOffline

	query := `
		INSERT INTO drivers (id, phone, name, email, license_number, vehicle_type, vehicle_number,
			status, rating, total_trips, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.ExecContext(ctx, query,
		driver.ID, driver.Phone, driver.Name, driver.Email, driver.LicenseNumber,
		driver.VehicleType, driver.VehicleNumber, driver.Status, driver.Rating,
		driver.TotalTrips, driver.CreatedAt, driver.UpdatedAt)
	return err
}

func (r *driverRepository) GetByID(ctx context.Context, id string) (*models.Driver, error) {
	var driver models.Driver
	query := `SELECT * FROM drivers WHERE id = $1`
	err := r.db.GetContext(ctx, &driver, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &driver, err
}

func (r *driverRepository) GetByPhone(ctx context.Context, phone string) (*models.Driver, error) {
	var driver models.Driver
	query := `SELECT * FROM drivers WHERE phone = $1`
	err := r.db.GetContext(ctx, &driver, query, phone)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &driver, err
}

func (r *driverRepository) Update(ctx context.Context, driver *models.Driver) error {
	driver.UpdatedAt = time.Now()
	query := `
		UPDATE drivers
		SET name = $1, email = $2, vehicle_type = $3, vehicle_number = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		driver.Name, driver.Email, driver.VehicleType, driver.VehicleNumber,
		driver.UpdatedAt, driver.ID)
	return err
}

func (r *driverRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE drivers SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

func (r *driverRepository) UpdateLocation(ctx context.Context, id string, lat, lng float64) error {
	query := `UPDATE drivers SET current_lat = $1, current_lng = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, lat, lng, time.Now(), id)
	return err
}

func (r *driverRepository) UpdateRating(ctx context.Context, id string, rating float64) error {
	query := `UPDATE drivers SET rating = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, rating, time.Now(), id)
	return err
}

func (r *driverRepository) IncrementTotalTrips(ctx context.Context, id string) error {
	query := `UPDATE drivers SET total_trips = total_trips + 1, updated_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

func (r *driverRepository) GetOnlineDriversByVehicleType(ctx context.Context, vehicleType string) ([]*models.Driver, error) {
	var drivers []*models.Driver
	query := `
		SELECT * FROM drivers
		WHERE status = $1 AND vehicle_type = $2
		AND current_lat IS NOT NULL AND current_lng IS NOT NULL
	`
	err := r.db.SelectContext(ctx, &drivers, query, models.DriverStatusOnline, vehicleType)
	return drivers, err
}
