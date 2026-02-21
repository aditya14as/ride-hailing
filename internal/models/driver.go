package models

import (
	"time"
)

// Driver status constants
const (
	DriverStatusOffline = "offline"
	DriverStatusOnline  = "online"
	DriverStatusBusy    = "busy"
)

// Vehicle types
const (
	VehicleTypeAuto  = "auto"
	VehicleTypeMini  = "mini"
	VehicleTypeSedan = "sedan"
	VehicleTypeSUV   = "suv"
)

type Driver struct {
	ID            string    `db:"id" json:"id"`
	Phone         string    `db:"phone" json:"phone"`
	Name          string    `db:"name" json:"name"`
	Email         *string   `db:"email" json:"email,omitempty"`
	LicenseNumber string    `db:"license_number" json:"license_number"`
	VehicleType   string    `db:"vehicle_type" json:"vehicle_type"`
	VehicleNumber string    `db:"vehicle_number" json:"vehicle_number"`
	Status        string    `db:"status" json:"status"`
	Rating        float64   `db:"rating" json:"rating"`
	TotalTrips    int       `db:"total_trips" json:"total_trips"`
	CurrentLat    *float64  `db:"current_lat" json:"current_lat,omitempty"`
	CurrentLng    *float64  `db:"current_lng" json:"current_lng,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

type CreateDriverRequest struct {
	Phone         string `json:"phone" validate:"required,min=10,max=15"`
	Name          string `json:"name" validate:"required,min=2,max=100"`
	Email         string `json:"email,omitempty" validate:"omitempty,email"`
	LicenseNumber string `json:"license_number" validate:"required"`
	VehicleType   string `json:"vehicle_type" validate:"required,oneof=auto mini sedan suv"`
	VehicleNumber string `json:"vehicle_number" validate:"required"`
}

type UpdateDriverLocationRequest struct {
	Lat      float64  `json:"lat" validate:"required,latitude"`
	Lng      float64  `json:"lng" validate:"required,longitude"`
	Heading  *float64 `json:"heading,omitempty"`
	Speed    *float64 `json:"speed,omitempty"`
	Accuracy *float64 `json:"accuracy,omitempty"`
}

type DriverResponse struct {
	ID            string   `json:"id"`
	Phone         string   `json:"phone"`
	Name          string   `json:"name"`
	Rating        float64  `json:"rating"`
	VehicleType   string   `json:"vehicle_type"`
	VehicleNumber string   `json:"vehicle_number"`
	Status        string   `json:"status"`
	CurrentLat    *float64 `json:"current_lat,omitempty"`
	CurrentLng    *float64 `json:"current_lng,omitempty"`
}

type DriverWithDistance struct {
	Driver   *Driver
	Distance float64 // in km
}

func (d *Driver) ToResponse() *DriverResponse {
	return &DriverResponse{
		ID:            d.ID,
		Phone:         d.Phone,
		Name:          d.Name,
		Rating:        d.Rating,
		VehicleType:   d.VehicleType,
		VehicleNumber: d.VehicleNumber,
		Status:        d.Status,
		CurrentLat:    d.CurrentLat,
		CurrentLng:    d.CurrentLng,
	}
}

func IsValidVehicleType(vt string) bool {
	return vt == VehicleTypeAuto || vt == VehicleTypeMini || vt == VehicleTypeSedan || vt == VehicleTypeSUV
}

func IsValidDriverStatus(status string) bool {
	return status == DriverStatusOffline || status == DriverStatusOnline || status == DriverStatusBusy
}
