package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/aditya/go-comet/internal/errors"
	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
	"github.com/google/uuid"
)

type PaymentService interface {
	ProcessPayment(ctx context.Context, req *models.CreatePaymentRequest) (*models.PaymentResponse, error)
	GetPayment(ctx context.Context, id string) (*models.Payment, error)
	GetPaymentByTripID(ctx context.Context, tripID string) (*models.Payment, error)
	RefundPayment(ctx context.Context, paymentID string) error
}

type paymentService struct {
	paymentRepo repository.PaymentRepository
	tripRepo    repository.TripRepository
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	tripRepo repository.TripRepository,
) PaymentService {
	return &paymentService{
		paymentRepo: paymentRepo,
		tripRepo:    tripRepo,
	}
}

func (s *paymentService) ProcessPayment(ctx context.Context, req *models.CreatePaymentRequest) (*models.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing.ToResponse(), nil
		}
	}

	// Get trip
	trip, err := s.tripRepo.GetByID(ctx, req.TripID)
	if err != nil {
		return nil, err
	}
	if trip == nil {
		return nil, apperrors.NotFound("trip")
	}

	// Verify trip is completed
	if trip.Status != models.TripStatusCompleted {
		return nil, apperrors.BadRequest("trip is not completed")
	}

	if trip.TotalFare == nil {
		return nil, apperrors.BadRequest("trip fare not calculated")
	}

	// Check if payment already exists for this trip
	existing, err := s.paymentRepo.GetByTripID(ctx, req.TripID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.Status == models.PaymentStatusCompleted {
			return existing.ToResponse(), nil
		}
	}

	// Create payment
	payment := &models.Payment{
		TripID:   trip.ID,
		UserID:   trip.UserID,
		DriverID: trip.DriverID,
		Amount:   *trip.TotalFare,
		Currency: "INR",
		Method:   req.Method,
		Status:   models.PaymentStatusPending,
	}

	if req.IdempotencyKey != "" {
		payment.IdempotencyKey = &req.IdempotencyKey
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	// Process payment based on method
	var pspResponse *PSPResponse
	var pspErr error

	switch req.Method {
	case models.PaymentMethodCash:
		pspResponse = s.processCashPayment(payment)
	case models.PaymentMethodWallet:
		pspResponse, pspErr = s.processWalletPayment(payment)
	case models.PaymentMethodCard, models.PaymentMethodUPI:
		pspResponse, pspErr = s.processExternalPayment(payment)
	default:
		return nil, apperrors.BadRequest("invalid payment method")
	}

	if pspErr != nil {
		// Update payment status to failed
		responseJSON, _ := json.Marshal(map[string]string{"error": pspErr.Error()})
		s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusFailed, nil, responseJSON)
		return nil, pspErr
	}

	// Update payment with PSP response
	pspTxnID := pspResponse.TransactionID
	responseJSON, _ := json.Marshal(pspResponse)
	if err := s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusCompleted, &pspTxnID, responseJSON); err != nil {
		return nil, err
	}

	payment.Status = models.PaymentStatusCompleted
	payment.PSPTransactionID = &pspTxnID

	return payment.ToResponse(), nil
}

func (s *paymentService) GetPayment(ctx context.Context, id string) (*models.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, apperrors.NotFound("payment")
	}
	return payment, nil
}

func (s *paymentService) GetPaymentByTripID(ctx context.Context, tripID string) (*models.Payment, error) {
	payment, err := s.paymentRepo.GetByTripID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, apperrors.NotFound("payment")
	}
	return payment, nil
}

func (s *paymentService) RefundPayment(ctx context.Context, paymentID string) error {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment == nil {
		return apperrors.NotFound("payment")
	}

	if payment.Status != models.PaymentStatusCompleted {
		return apperrors.BadRequest("can only refund completed payments")
	}

	// Mock refund
	refundResponse := map[string]interface{}{
		"refund_id":   fmt.Sprintf("REF_%s", uuid.New().String()[:8]),
		"refunded_at": time.Now().Format(time.RFC3339),
	}
	responseJSON, _ := json.Marshal(refundResponse)

	return s.paymentRepo.UpdateStatus(ctx, paymentID, models.PaymentStatusRefunded, payment.PSPTransactionID, responseJSON)
}

// PSP Response types (mock)
type PSPResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	ProcessedAt   string `json:"processed_at"`
}

// Mock payment processors
func (s *paymentService) processCashPayment(payment *models.Payment) *PSPResponse {
	// Cash payments are marked as completed immediately
	return &PSPResponse{
		TransactionID: fmt.Sprintf("CASH_%s", uuid.New().String()[:8]),
		Status:        "success",
		Message:       "Cash payment collected",
		ProcessedAt:   time.Now().Format(time.RFC3339),
	}
}

func (s *paymentService) processWalletPayment(payment *models.Payment) (*PSPResponse, error) {
	// Mock wallet payment - always succeeds
	// In real implementation, check wallet balance and deduct
	return &PSPResponse{
		TransactionID: fmt.Sprintf("WAL_%s", uuid.New().String()[:8]),
		Status:        "success",
		Message:       "Wallet payment successful",
		ProcessedAt:   time.Now().Format(time.RFC3339),
	}, nil
}

func (s *paymentService) processExternalPayment(payment *models.Payment) (*PSPResponse, error) {
	// Mock external PSP (card/UPI) payment
	// In real implementation, call payment gateway API
	return &PSPResponse{
		TransactionID: fmt.Sprintf("PSP_%s", uuid.New().String()[:8]),
		Status:        "success",
		Message:       "Payment successful via " + payment.Method,
		ProcessedAt:   time.Now().Format(time.RFC3339),
	}, nil
}
