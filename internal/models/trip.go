package models

import (
	"time"
)

// Trip status constants
const (
	TripStatusStarted   = "started"
	TripStatusPaused    = "paused"
	TripStatusCompleted = "completed"
	TripStatusCancelled = "cancelled"
)

// Valid trip state transitions
var ValidTripTransitions = map[string][]string{
	TripStatusStarted:   {TripStatusPaused, TripStatusCompleted, TripStatusCancelled},
	TripStatusPaused:    {TripStatusStarted, TripStatusCompleted, TripStatusCancelled},
	TripStatusCompleted: {},
	TripStatusCancelled: {},
}

type Trip struct {
	ID                string     `db:"id" json:"id"`
	RideID            string     `db:"ride_id" json:"ride_id"`
	DriverID          string     `db:"driver_id" json:"driver_id"`
	UserID            string     `db:"user_id" json:"user_id"`
	Status            string     `db:"status" json:"status"`
	StartTime         *time.Time `db:"start_time" json:"start_time,omitempty"`
	EndTime           *time.Time `db:"end_time" json:"end_time,omitempty"`
	PauseDurationSecs int        `db:"pause_duration_secs" json:"pause_duration_secs"`
	ActualDistanceKm  *float64   `db:"actual_distance_km" json:"actual_distance_km,omitempty"`
	ActualDurationMin *int       `db:"actual_duration_mins" json:"actual_duration_mins,omitempty"`
	RoutePolyline     *string    `db:"route_polyline" json:"route_polyline,omitempty"`
	BaseFare          *float64   `db:"base_fare" json:"base_fare,omitempty"`
	DistanceFare      *float64   `db:"distance_fare" json:"distance_fare,omitempty"`
	TimeFare          *float64   `db:"time_fare" json:"time_fare,omitempty"`
	SurgeAmount       *float64   `db:"surge_amount" json:"surge_amount,omitempty"`
	TotalFare         *float64   `db:"total_fare" json:"total_fare,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

type FareBreakdown struct {
	BaseFare     float64 `json:"base_fare"`
	DistanceFare float64 `json:"distance_fare"`
	TimeFare     float64 `json:"time_fare"`
	SurgeAmount  float64 `json:"surge_amount"`
	Total        float64 `json:"total"`
}

type EndTripRequest struct {
	EndLat     float64  `json:"end_lat" validate:"required,latitude"`
	EndLng     float64  `json:"end_lng" validate:"required,longitude"`
	OdometerKm *float64 `json:"odometer_km,omitempty"`
}

type TripResponse struct {
	ID                string         `json:"id"`
	RideID            string         `json:"ride_id"`
	Status            string         `json:"status"`
	StartTime         *time.Time     `json:"start_time,omitempty"`
	EndTime           *time.Time     `json:"end_time,omitempty"`
	ActualDistanceKm  *float64       `json:"actual_distance_km,omitempty"`
	ActualDurationMin *int           `json:"actual_duration_mins,omitempty"`
	FareBreakdown     *FareBreakdown `json:"fare_breakdown,omitempty"`
}

func (t *Trip) ToResponse() *TripResponse {
	resp := &TripResponse{
		ID:                t.ID,
		RideID:            t.RideID,
		Status:            t.Status,
		StartTime:         t.StartTime,
		EndTime:           t.EndTime,
		ActualDistanceKm:  t.ActualDistanceKm,
		ActualDurationMin: t.ActualDurationMin,
	}

	if t.TotalFare != nil {
		resp.FareBreakdown = &FareBreakdown{
			BaseFare:     ptrToFloat(t.BaseFare),
			DistanceFare: ptrToFloat(t.DistanceFare),
			TimeFare:     ptrToFloat(t.TimeFare),
			SurgeAmount:  ptrToFloat(t.SurgeAmount),
			Total:        *t.TotalFare,
		}
	}

	return resp
}

// CanTransitionTo checks if a trip can transition to a new status
func (t *Trip) CanTransitionTo(newStatus string) bool {
	validNextStates, exists := ValidTripTransitions[t.Status]
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

func ptrToFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
