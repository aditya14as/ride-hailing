package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors
var (
	ErrNotFound            = errors.New("resource not found")
	ErrConflict            = errors.New("resource conflict")
	ErrBadRequest          = errors.New("bad request")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrForbidden           = errors.New("forbidden")
	ErrInternalServer      = errors.New("internal server error")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")

	// Business errors
	ErrNoDriversAvailable  = errors.New("no drivers available")
	ErrRideAlreadyAssigned = errors.New("ride already assigned")
	ErrOfferExpired        = errors.New("offer expired")
	ErrInvalidTransition   = errors.New("invalid state transition")
	ErrUserHasActiveRide   = errors.New("user already has an active ride")
	ErrDriverBusy          = errors.New("driver is busy")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrPaymentFailed       = errors.New("payment failed")
)

// APIError represents a structured API error
type APIError struct {
	Code       string `json:"error"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new API error
func NewAPIError(code, message string, statusCode int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Common API errors
func NotFound(resource string) *APIError {
	return NewAPIError("not_found", fmt.Sprintf("%s not found", resource), http.StatusNotFound)
}

func BadRequest(message string) *APIError {
	return NewAPIError("bad_request", message, http.StatusBadRequest)
}

func Conflict(message string) *APIError {
	return NewAPIError("conflict", message, http.StatusConflict)
}

func InternalError(message string) *APIError {
	return NewAPIError("internal_error", message, http.StatusInternalServerError)
}

func Unauthorized(message string) *APIError {
	return NewAPIError("unauthorized", message, http.StatusUnauthorized)
}

func IdempotencyConflict() *APIError {
	return NewAPIError("idempotency_conflict", "idempotency key already used with different request", http.StatusConflict)
}

func NoDriversAvailable() *APIError {
	return NewAPIError("no_drivers_available", "no drivers available in your area", http.StatusServiceUnavailable)
}

func RideAlreadyAssigned() *APIError {
	return NewAPIError("ride_already_assigned", "this ride has been assigned to another driver", http.StatusConflict)
}

func OfferExpired() *APIError {
	return NewAPIError("offer_expired", "this ride offer has expired", http.StatusGone)
}

func InvalidTransition(from, to string) *APIError {
	return NewAPIError("invalid_transition", fmt.Sprintf("cannot transition from %s to %s", from, to), http.StatusBadRequest)
}

func UserHasActiveRide() *APIError {
	return NewAPIError("active_ride_exists", "you already have an active ride", http.StatusConflict)
}

func InsufficientFunds() *APIError {
	return NewAPIError("insufficient_funds", "wallet balance insufficient", http.StatusPaymentRequired)
}
