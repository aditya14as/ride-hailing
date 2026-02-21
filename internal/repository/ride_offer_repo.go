package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RideOfferRepository interface {
	Create(ctx context.Context, offer *models.RideOffer) error
	GetByID(ctx context.Context, id string) (*models.RideOffer, error)
	GetByRideAndDriver(ctx context.Context, rideID, driverID string) (*models.RideOffer, error)
	GetPendingByRideID(ctx context.Context, rideID string) ([]*models.RideOffer, error)
	GetPendingByDriverID(ctx context.Context, driverID string) ([]*models.RideOffer, error)
	UpdateStatus(ctx context.Context, id, status string) error
	ExpireOldOffers(ctx context.Context, rideID string) error
	GetByIDForUpdate(ctx context.Context, tx *sqlx.Tx, id string) (*models.RideOffer, error)
}

type rideOfferRepository struct {
	db *sqlx.DB
}

func NewRideOfferRepository(db *sqlx.DB) RideOfferRepository {
	return &rideOfferRepository{db: db}
}

func (r *rideOfferRepository) Create(ctx context.Context, offer *models.RideOffer) error {
	if offer.ID == "" {
		offer.ID = uuid.New().String()
	}
	offer.OfferedAt = time.Now()
	offer.Status = models.OfferStatusPending

	query := `
		INSERT INTO ride_offers (id, ride_id, driver_id, status, offered_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		offer.ID, offer.RideID, offer.DriverID, offer.Status, offer.OfferedAt, offer.ExpiresAt)
	return err
}

func (r *rideOfferRepository) GetByID(ctx context.Context, id string) (*models.RideOffer, error) {
	var offer models.RideOffer
	query := `SELECT * FROM ride_offers WHERE id = $1`
	err := r.db.GetContext(ctx, &offer, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &offer, err
}

func (r *rideOfferRepository) GetByRideAndDriver(ctx context.Context, rideID, driverID string) (*models.RideOffer, error) {
	var offer models.RideOffer
	query := `SELECT * FROM ride_offers WHERE ride_id = $1 AND driver_id = $2`
	err := r.db.GetContext(ctx, &offer, query, rideID, driverID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &offer, err
}

func (r *rideOfferRepository) GetPendingByRideID(ctx context.Context, rideID string) ([]*models.RideOffer, error) {
	var offers []*models.RideOffer
	query := `
		SELECT * FROM ride_offers
		WHERE ride_id = $1 AND status = $2 AND expires_at > NOW()
		ORDER BY offered_at ASC
	`
	err := r.db.SelectContext(ctx, &offers, query, rideID, models.OfferStatusPending)
	return offers, err
}

func (r *rideOfferRepository) GetPendingByDriverID(ctx context.Context, driverID string) ([]*models.RideOffer, error) {
	var offers []*models.RideOffer
	query := `
		SELECT * FROM ride_offers
		WHERE driver_id = $1 AND status = $2 AND expires_at > NOW()
		ORDER BY offered_at DESC
	`
	err := r.db.SelectContext(ctx, &offers, query, driverID, models.OfferStatusPending)
	return offers, err
}

func (r *rideOfferRepository) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now()
	query := `UPDATE ride_offers SET status = $1, responded_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, now, id)
	return err
}

func (r *rideOfferRepository) ExpireOldOffers(ctx context.Context, rideID string) error {
	query := `
		UPDATE ride_offers
		SET status = $1, responded_at = NOW()
		WHERE ride_id = $2 AND status = $3
	`
	_, err := r.db.ExecContext(ctx, query, models.OfferStatusExpired, rideID, models.OfferStatusPending)
	return err
}

func (r *rideOfferRepository) GetByIDForUpdate(ctx context.Context, tx *sqlx.Tx, id string) (*models.RideOffer, error) {
	var offer models.RideOffer
	query := `SELECT * FROM ride_offers WHERE id = $1 FOR UPDATE`
	err := tx.GetContext(ctx, &offer, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &offer, err
}
