package models

import (
	"encoding/json"
	"time"
)

// Payment status constants
const (
	PaymentStatusPending    = "pending"
	PaymentStatusProcessing = "processing"
	PaymentStatusCompleted  = "completed"
	PaymentStatusFailed     = "failed"
	PaymentStatusRefunded   = "refunded"
)

type Payment struct {
	ID               string          `db:"id" json:"id"`
	TripID           string          `db:"trip_id" json:"trip_id"`
	UserID           string          `db:"user_id" json:"user_id"`
	DriverID         string          `db:"driver_id" json:"driver_id"`
	Amount           float64         `db:"amount" json:"amount"`
	Currency         string          `db:"currency" json:"currency"`
	Method           string          `db:"method" json:"method"`
	Status           string          `db:"status" json:"status"`
	PSPTransactionID *string         `db:"psp_transaction_id" json:"psp_transaction_id,omitempty"`
	PSPResponse      json.RawMessage `db:"psp_response" json:"psp_response,omitempty"`
	IdempotencyKey   *string         `db:"idempotency_key" json:"idempotency_key,omitempty"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at" json:"updated_at"`
}

type CreatePaymentRequest struct {
	TripID         string `json:"trip_id" validate:"required,uuid"`
	Method         string `json:"method" validate:"required,oneof=cash wallet card upi"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type PaymentResponse struct {
	ID            string  `json:"id"`
	TripID        string  `json:"trip_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Method        string  `json:"method"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id,omitempty"`
}

func (p *Payment) ToResponse() *PaymentResponse {
	return &PaymentResponse{
		ID:            p.ID,
		TripID:        p.TripID,
		Amount:        p.Amount,
		Currency:      p.Currency,
		Method:        p.Method,
		Status:        p.Status,
		TransactionID: p.PSPTransactionID,
	}
}
