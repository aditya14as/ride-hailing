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

type PaymentHandler struct {
	paymentService service.PaymentService
	validate       *validator.Validate
}

func NewPaymentHandler(paymentService service.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		validate:       validator.New(),
	}
}

func (h *PaymentHandler) RegisterRoutes(r chi.Router) {
	r.Post("/payments", h.ProcessPayment)
	r.Get("/payments/{id}", h.GetPayment)
	r.Post("/payments/{id}/refund", h.RefundPayment)
}

// POST /v1/payments
func (h *PaymentHandler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	var req models.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	payment, err := h.paymentService.ProcessPayment(r.Context(), &req)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, payment)
}

// GET /v1/payments/{id}
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "payment id is required")
		return
	}

	payment, err := h.paymentService.GetPayment(r.Context(), id)
	if err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, payment.ToResponse())
}

// POST /v1/payments/{id}/refund
func (h *PaymentHandler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "payment id is required")
		return
	}

	if err := h.paymentService.RefundPayment(r.Context(), id); err != nil {
		handleError(w, err)
		return
	}

	utils.Success(w, http.StatusOK, map[string]string{
		"status":  "refunded",
		"message": "payment refunded successfully",
	})
}
