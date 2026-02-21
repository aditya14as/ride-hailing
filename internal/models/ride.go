package models

import (
	"time"
)

// Ride status constants
const (
	RideStatusPending        = "pending"
	RideStatusMatching       = "matching"
	RideStatusDriverAssigned = "driver_assigned"
	RideStatusDriverArrived  = "driver_arrived"
	RideStatusInProgress     = "in_progress"
	RideStatusCompleted      = "completed"
	RideStatusCancelled      = "cancelled"
)

// Valid ride state transitions
var ValidRideTransitions = map[string][]string{
	RideStatusPending:        {RideStatusMatching, RideStatusCancelled},
	RideStatusMatching:       {RideStatusDriverAssigned, RideStatusCancelled},
	RideStatusDriverAssigned: {RideStatusDriverArrived, RideStatusCancelled},
	RideStatusDriverArrived:  {RideStatusInProgress, RideStatusCancelled},
	RideStatusInProgress:     {RideStatusCompleted, RideStatusCancelled},
	RideStatusCompleted:      {},
	RideStatusCancelled:      {},
}

// Payment methods
const (
	PaymentMethodCash   = "cash"
	PaymentMethodWallet = "wallet"
	PaymentMethodCard   = "card"
	PaymentMethodUPI    = "upi"
)

type Location struct {
	Lat     float64 `json:"lat" validate:"required,latitude"`
	Lng     float64 `json:"lng" validate:"required,longitude"`
	Address string  `json:"address,omitempty"`
}

type Ride struct {
	ID                   string    `db:"id" json:"id"`
	UserID               string    `db:"user_id" json:"user_id"`
	DriverID             *string   `db:"driver_id" json:"driver_id,omitempty"`
	PickupLat            float64   `db:"pickup_lat" json:"pickup_lat"`
	PickupLng            float64   `db:"pickup_lng" json:"pickup_lng"`
	PickupAddress        *string   `db:"pickup_address" json:"pickup_address,omitempty"`
	DropoffLat           float64   `db:"dropoff_lat" json:"dropoff_lat"`
	DropoffLng           float64   `db:"dropoff_lng" json:"dropoff_lng"`
	DropoffAddress       *string   `db:"dropoff_address" json:"dropoff_address,omitempty"`
	VehicleType          string    `db:"vehicle_type" json:"vehicle_type"`
	Status               string    `db:"status" json:"status"`
	EstimatedFare        *float64  `db:"estimated_fare" json:"estimated_fare,omitempty"`
	SurgeMultiplier      float64   `db:"surge_multiplier" json:"surge_multiplier"`
	EstimatedDistanceKm  *float64  `db:"estimated_distance_km" json:"estimated_distance_km,omitempty"`
	EstimatedDurationMin *int      `db:"estimated_duration_mins" json:"estimated_duration_mins,omitempty"`
	PaymentMethod        string    `db:"payment_method" json:"payment_method"`
	IdempotencyKey       *string   `db:"idempotency_key" json:"idempotency_key,omitempty"`
	CancelledBy          *string   `db:"cancelled_by" json:"cancelled_by,omitempty"`
	CancellationReason   *string   `db:"cancellation_reason" json:"cancellation_reason,omitempty"`
	CreatedAt            time.Time `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time `db:"updated_at" json:"updated_at"`
}

type CreateRideRequest struct {
	UserID        string   `json:"user_id" validate:"required,uuid"`
	Pickup        Location `json:"pickup" validate:"required"`
	Dropoff       Location `json:"dropoff" validate:"required"`
	VehicleType   string   `json:"vehicle_type" validate:"required,oneof=auto mini sedan suv"`
	PaymentMethod string   `json:"payment_method" validate:"required,oneof=cash wallet card upi"`
}

type RideResponse struct {
	ID                   string           `json:"id"`
	Status               string           `json:"status"`
	User                 *UserResponse    `json:"user,omitempty"`
	Driver               *DriverResponse  `json:"driver,omitempty"`
	Pickup               Location         `json:"pickup"`
	Dropoff              Location         `json:"dropoff"`
	VehicleType          string           `json:"vehicle_type"`
	EstimatedFare        *float64         `json:"estimated_fare,omitempty"`
	SurgeMultiplier      float64          `json:"surge_multiplier"`
	EstimatedDistanceKm  *float64         `json:"estimated_distance_km,omitempty"`
	EstimatedDurationMin *int             `json:"estimated_duration_mins,omitempty"`
	PaymentMethod        string           `json:"payment_method"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

type CancelRideRequest struct {
	Reason      string `json:"reason,omitempty"`
	CancelledBy string `json:"cancelled_by" validate:"required,oneof=user driver system"`
}

func (r *Ride) ToResponse() *RideResponse {
	resp := &RideResponse{
		ID:     r.ID,
		Status: r.Status,
		Pickup: Location{
			Lat: r.PickupLat,
			Lng: r.PickupLng,
		},
		Dropoff: Location{
			Lat: r.DropoffLat,
			Lng: r.DropoffLng,
		},
		VehicleType:          r.VehicleType,
		EstimatedFare:        r.EstimatedFare,
		SurgeMultiplier:      r.SurgeMultiplier,
		EstimatedDistanceKm:  r.EstimatedDistanceKm,
		EstimatedDurationMin: r.EstimatedDurationMin,
		PaymentMethod:        r.PaymentMethod,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}

	if r.PickupAddress != nil {
		resp.Pickup.Address = *r.PickupAddress
	}
	if r.DropoffAddress != nil {
		resp.Dropoff.Address = *r.DropoffAddress
	}

	return resp
}

// CanTransitionTo checks if a ride can transition to a new status
func (r *Ride) CanTransitionTo(newStatus string) bool {
	validNextStates, exists := ValidRideTransitions[r.Status]
	if !exists {
		return false
	}

	for _, state := range validNextStates {
		if state == newStatus {
			return true
		}
	}
	return false
}

// IsActive returns true if the ride is not in a terminal state
func (r *Ride) IsActive() bool {
	return r.Status != RideStatusCompleted && r.Status != RideStatusCancelled
}
