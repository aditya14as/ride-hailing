package models

import (
	"time"
)

type User struct {
	ID        string    `db:"id" json:"id"`
	Phone     string    `db:"phone" json:"phone"`
	Name      string    `db:"name" json:"name"`
	Email     *string   `db:"email" json:"email,omitempty"`
	Rating    float64   `db:"rating" json:"rating"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type CreateUserRequest struct {
	Phone string `json:"phone" validate:"required,min=10,max=15"`
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email,omitempty" validate:"omitempty,email"`
}

type UserResponse struct {
	ID     string  `json:"id"`
	Phone  string  `json:"phone"`
	Name   string  `json:"name"`
	Email  *string `json:"email,omitempty"`
	Rating float64 `json:"rating"`
}

func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:     u.ID,
		Phone:  u.Phone,
		Name:   u.Name,
		Email:  u.Email,
		Rating: u.Rating,
	}
}
