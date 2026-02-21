package utils

import "github.com/google/uuid"

// GenerateID generates a new UUID v4
func GenerateID() string {
	return uuid.New().String()
}

// IsValidUUID checks if a string is a valid UUID
func IsValidUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
