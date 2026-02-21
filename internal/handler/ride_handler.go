package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/middleware"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/service"
	"github.com/aditya/go-comet/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type RideHandler struct {
	rideService     service.RideService
	matchingService service.MatchingService
	validate        *validator.Validate
}

func NewRideHandler(rideService service.RideService, matchingService service.MatchingService) *RideHandler {
	return &RideHandler{
		rideService:     rideService,
		matchingService: matchingService,
		validate:        validator.New(),
	}
}

func (h *RideHandler) RegisterRoutes(r chi.Router) {
	r.Post("/rides", h.CreateRide)
	r.Get("/rides/{id}", h.GetRide)
	r.Post("/rides/{id}/cancel", h.CancelRide)
}

// POST /v1/rides
func (h *RideHandler) CreateRide(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	idempotencyKey := r.Header.Get(middleware.IdempotencyHeader)

	ride, err := h.rideService.CreateRide(r.Context(), &req, idempotencyKey)
	if err != nil {
		handleError(w, err)
		return
	}

	// Trigger matching asynchronously
	go func() {
		if err := h.matchingService.FindAndOfferDrivers(r.Context(), ride); err != nil {
			// Log error, don't fail the request
		}
	}()

	utils.Created(w, ride)
}

// GET /v1/rides/{id}
func (h *RideHandler) GetRide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "ride id is required")
		return
	}

	ride, err := h.rideService.GetRide(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, ride)
}

// POST /v1/rides/{id}/cancel
func (h *RideHandler) CancelRide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "ride id is required")
		return
	}

	var req models.CancelRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	if err := h.rideService.CancelRide(r.Context(), id, &req); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status":  "cancelled",
		"message": "ride cancelled successfully",
	})
}

func handleError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*apperrors.APIError); ok {
		utils.Error(w, apiErr)
		return
	}

	// Check for specific errors
	switch err {
	case apperrors.ErrNoDriversAvailable:
		utils.Error(w, apperrors.NoDriversAvailable())
	case apperrors.ErrRideAlreadyAssigned:
		utils.Error(w, apperrors.RideAlreadyAssigned())
	case apperrors.ErrOfferExpired:
		utils.Error(w, apperrors.OfferExpired())
	case apperrors.ErrUserHasActiveRide:
		utils.Error(w, apperrors.UserHasActiveRide())
	default:
		utils.InternalError(w, "internal server error")
	}
}
