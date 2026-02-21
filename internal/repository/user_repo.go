package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/aditya/go-comet/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByPhone(ctx context.Context, phone string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateRating(ctx context.Context, id string, rating float64) error
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.Rating = 5.0

	query := `
		INSERT INTO users (id, phone, name, email, rating, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Phone, user.Name, user.Email, user.Rating, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = $1`
	err := r.db.GetContext(ctx, &user, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE phone = $1`
	err := r.db.GetContext(ctx, &user, query, phone)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &user, err
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	query := `
		UPDATE users
		SET name = $1, email = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, user.Name, user.Email, user.UpdatedAt, user.ID)
	return err
}

func (r *userRepository) UpdateRating(ctx context.Context, id string, rating float64) error {
	query := `UPDATE users SET rating = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, rating, time.Now(), id)
	return err
}
