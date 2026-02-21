package handler

import (
	"encoding/json"
	"net/http"

	"github.com/aditya/go-comet/internal/models"
	"github.com/aditya/go-comet/internal/repository"
	"github.com/aditya/go-comet/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userRepo repository.UserRepository
	validate *validator.Validate
}

func NewUserHandler(userRepo repository.UserRepository) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
		validate: validator.New(),
	}
}

func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Post("/users", h.CreateUser)
	r.Get("/users/{id}", h.GetUser)
}

// POST /v1/users
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.BadRequest(w, "invalid request body")
		return
	}

	if err := h.validate.Struct(req); err != nil {
		utils.BadRequest(w, err.Error())
		return
	}

	// Check if phone already exists
	existing, err := h.userRepo.GetByPhone(r.Context(), req.Phone)
	if err != nil {
		utils.InternalError(w, "failed to check existing user")
		return
	}
	if existing != nil {
		utils.JSON(w, http.StatusConflict, map[string]string{
			"error":   "conflict",
			"message": "user with this phone already exists",
		})
		return
	}

	user := &models.User{
		Phone: req.Phone,
		Name:  req.Name,
	}

	if req.Email != "" {
		user.Email = &req.Email
	}

	if err := h.userRepo.Create(r.Context(), user); err != nil {
		utils.InternalError(w, "failed to create user")
		return
	}

	utils.Created(w, user.ToResponse())
}

// GET /v1/users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.BadRequest(w, "user id is required")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		utils.InternalError(w, "failed to get user")
		return
	}
	if user == nil {
		utils.NotFound(w, "user")
		return
	}

	utils.Success(w, http.StatusOK, user.ToResponse())
}
