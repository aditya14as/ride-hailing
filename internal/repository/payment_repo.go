package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *models.Payment) error
	GetByID(ctx context.Context, id string) (*models.Payment, error)
	GetByTripID(ctx context.Context, tripID string) (*models.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error)
	Update(ctx context.Context, payment *models.Payment) error
	UpdateStatus(ctx context.Context, id, status string, pspTxnID *string, pspResponse json.RawMessage) error
}

type paymentRepository struct {
	db *sqlx.DB
}

func NewPaymentRepository(db *sqlx.DB) PaymentRepository {
	return &paymentRepository{db: db}
}

func (r *paymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	if payment.ID == "" {
		payment.ID = uuid.New().String()
	}
	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()
	payment.Status = models.PaymentStatusPending
	if payment.Currency == "" {
		payment.Currency = "INR"
	}

	query := `
		INSERT INTO payments (id, trip_id, user_id, driver_id, amount, currency,
			method, status, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, query,
		payment.ID, payment.TripID, payment.UserID, payment.DriverID,
		payment.Amount, payment.Currency, payment.Method, payment.Status,
		payment.IdempotencyKey, payment.CreatedAt, payment.UpdatedAt)
	return err
}

func (r *paymentRepository) GetByID(ctx context.Context, id string) (*models.Payment, error) {
	var payment models.Payment
	query := `SELECT * FROM payments WHERE id = $1`
	err := r.db.GetContext(ctx, &payment, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &payment, err
}

func (r *paymentRepository) GetByTripID(ctx context.Context, tripID string) (*models.Payment, error) {
	var payment models.Payment
	query := `SELECT * FROM payments WHERE trip_id = $1 ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &payment, query, tripID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &payment, err
}

func (r *paymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error) {
	var payment models.Payment
	query := `SELECT * FROM payments WHERE idempotency_key = $1`
	err := r.db.GetContext(ctx, &payment, query, key)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &payment, err
}

func (r *paymentRepository) Update(ctx context.Context, payment *models.Payment) error {
	payment.UpdatedAt = time.Now()
	query := `
		UPDATE payments
		SET status = $1, psp_transaction_id = $2, psp_response = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query,
		payment.Status, payment.PSPTransactionID, payment.PSPResponse,
		payment.UpdatedAt, payment.ID)
	return err
}

func (r *paymentRepository) UpdateStatus(ctx context.Context, id, status string, pspTxnID *string, pspResponse json.RawMessage) error {
	query := `
		UPDATE payments
		SET status = $1, psp_transaction_id = $2, psp_response = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query, status, pspTxnID, pspResponse, time.Now(), id)
	return err
}
