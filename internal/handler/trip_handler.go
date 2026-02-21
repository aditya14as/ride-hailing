package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/service"
	"github.com/aditya/go-comet/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type TripHandler struct {
	tripService service.TripService
	validate    *validator.Validate
}

func NewTripHandler(tripService service.TripService) *TripHandler {
	return &TripHandler{
		tripService: tripService,
		validate:    validator.New(),
	}
}

func (h *TripHandler) RegisterRoutes(r chi.Router) {
	r.Post("/trips/start", h.StartTrip)
	r.Get("/trips/{id}", h.GetTrip)
	r.Post("/trips/{id}/end", h.EndTrip)
	r.Post("/trips/{id}/pause", h.PauseTrip)
	r.Post("/trips/{id}/resume", h.ResumeTrip)
}

// POST /v1/trips/start
func (h *TripHandler) StartTrip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RideID string `json:"ride_id" validate:"required,uuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	trip, err := h.tripService.StartTrip(r.Context(), req.RideID)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Created(w, trip.ToResponse())
}

// GET /v1/trips/{id}
func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "trip id is required")
		return
	}

	trip, err := h.tripService.GetTrip(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, trip.ToResponse())
}

// POST /v1/trips/{id}/end
func (h *TripHandler) EndTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "trip id is required")
		return
	}

	var req models.EndTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	trip, err := h.tripService.EndTrip(r.Context(), id, &req)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]interface{}{
		"trip_id":        trip.ID,
		"status":         trip.Status,
		"fare_breakdown": trip.FareBreakdown,
	})
}

// POST /v1/trips/{id}/pause
func (h *TripHandler) PauseTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "trip id is required")
		return
	}

	if err := h.tripService.PauseTrip(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status": "paused",
	})
}

// POST /v1/trips/{id}/resume
func (h *TripHandler) ResumeTrip(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "trip id is required")
		return
	}

	if err := h.tripService.ResumeTrip(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status": "resumed",
	})
}
