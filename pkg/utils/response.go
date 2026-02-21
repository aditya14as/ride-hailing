package utils

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/aditya/go-comet/internal/errors"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON sends a JSON response
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Success sends a success response
func Success(w http.ResponseWriter, status int, data interface{}) {
	JSON(w, status, data)
}

// Error sends an error response
func Error(w http.ResponseWriter, err *apperrors.APIError) {
	JSON(w, err.StatusCode, map[string]string{
		"error":   err.Code,
		"message": err.Message,
	})
}

// BadRequest sends a 400 error
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, apperrors.BadRequest(message))
}

// NotFound sends a 404 error
func NotFound(w http.ResponseWriter, resource string) {
	Error(w, apperrors.NotFound(resource))
}

// InternalError sends a 500 error
func InternalError(w http.ResponseWriter, message string) {
	Error(w, apperrors.InternalError(message))
}

// Created sends a 201 response
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, data)
}

// NoContent sends a 204 response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
