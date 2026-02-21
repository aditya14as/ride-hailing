package models

import (
	"time"
)

// Ride offer status constants
const (
	OfferStatusPending  = "pending"
	OfferStatusAccepted = "accepted"
	OfferStatusDeclined = "declined"
	OfferStatusExpired  = "expired"
)

type RideOffer struct {
	ID          string     `db:"id" json:"id"`
	RideID      string     `db:"ride_id" json:"ride_id"`
	DriverID    string     `db:"driver_id" json:"driver_id"`
	Status      string     `db:"status" json:"status"`
	OfferedAt   time.Time  `db:"offered_at" json:"offered_at"`
	RespondedAt *time.Time `db:"responded_at" json:"responded_at,omitempty"`
	ExpiresAt   time.Time  `db:"expires_at" json:"expires_at"`
}

type AcceptRideRequest struct {
	RideID  string `json:"ride_id" validate:"required,uuid"`
	OfferID string `json:"offer_id" validate:"required,uuid"`
}

type RideOfferResponse struct {
	ID        string    `json:"id"`
	RideID    string    `json:"ride_id"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	Ride      *RideResponse `json:"ride,omitempty"`
}

func (o *RideOffer) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

func (o *RideOffer) ToResponse() *RideOfferResponse {
	return &RideOfferResponse{
		ID:        o.ID,
		RideID:    o.RideID,
		Status:    o.Status,
		ExpiresAt: o.ExpiresAt,
	}
}
