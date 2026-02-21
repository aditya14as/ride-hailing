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

type DriverHandler struct {
	driverService   service.DriverService
	matchingService service.MatchingService
	validate        *validator.Validate
}

func NewDriverHandler(driverService service.DriverService, matchingService service.MatchingService) *DriverHandler {
	return &DriverHandler{
		driverService:   driverService,
		matchingService: matchingService,
		validate:        validator.New(),
	}
}

func (h *DriverHandler) RegisterRoutes(r chi.Router) {
	r.Post("/drivers", h.CreateDriver)
	r.Get("/drivers/{id}", h.GetDriver)
	r.Post("/drivers/{id}/location", h.UpdateLocation)
	r.Post("/drivers/{id}/accept", h.AcceptRide)
	r.Post("/drivers/{id}/decline", h.DeclineRide)
	r.Post("/drivers/{id}/online", h.GoOnline)
	r.Post("/drivers/{id}/offline", h.GoOffline)
	r.Get("/drivers/{id}/offers", h.GetPendingOffers)
}

// POST /v1/drivers
func (h *DriverHandler) CreateDriver(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	driver, err := h.driverService.CreateDriver(r.Context(), &req)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Created(w, driver.ToResponse())
}

// GET /v1/drivers/{id}
func (h *DriverHandler) GetDriver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	driver, err := h.driverService.GetDriver(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, driver.ToResponse())
}

// POST /v1/drivers/{id}/location
func (h *DriverHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	var req models.UpdateDriverLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	if err := h.driverService.UpdateLocation(r.Context(), id, &req); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": r.Context().Value("request_time"),
	})
}

// POST /v1/drivers/{id}/accept
func (h *DriverHandler) AcceptRide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	var req models.AcceptRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	ride, err := h.driverService.AcceptRide(r.Context(), id, &req)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]interface{}{
		"status": "accepted",
		"ride":   ride,
	})
}

// POST /v1/drivers/{id}/decline
func (h *DriverHandler) DeclineRide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	var req struct {
		OfferID string `json:"offer_id" validate:"required,uuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.driverService.DeclineRide(r.Context(), id, req.OfferID); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status": "declined",
	})
}

// POST /v1/drivers/{id}/online
func (h *DriverHandler) GoOnline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	if err := h.driverService.GoOnline(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status": "online",
	})
}

// POST /v1/drivers/{id}/offline
func (h *DriverHandler) GoOffline(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	if err := h.driverService.GoOffline(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status": "offline",
	})
}

// GET /v1/drivers/{id}/offers
func (h *DriverHandler) GetPendingOffers(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "driver id is required")
		return
	}

	offers, err := h.matchingService.GetPendingOffers(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]interface{}{
		"offers": offers,
	})
}
