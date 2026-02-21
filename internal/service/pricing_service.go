package service

import (
	"math"

	"github.com/aditya/go-comet/internal/models"
)

// FareConfig holds pricing configuration for each vehicle type
type FareConfig struct {
	BaseFare        float64
	PerKmRate       float64
	PerMinRate      float64
	MinFare         float64
	CancellationFee float64
}

var fareConfigs = map[string]FareConfig{
	models.VehicleTypeAuto:  {BaseFare: 25, PerKmRate: 12, PerMinRate: 1.0, MinFare: 30, CancellationFee: 25},
	models.VehicleTypeMini:  {BaseFare: 40, PerKmRate: 14, PerMinRate: 1.2, MinFare: 50, CancellationFee: 40},
	models.VehicleTypeSedan: {BaseFare: 50, PerKmRate: 17, PerMinRate: 1.5, MinFare: 80, CancellationFee: 50},
	models.VehicleTypeSUV:   {BaseFare: 80, PerKmRate: 22, PerMinRate: 2.0, MinFare: 120, CancellationFee: 80},
}

type PricingService interface {
	CalculateEstimatedFare(vehicleType string, distanceKm float64, durationMins int, surgeMultiplier float64) *models.FareBreakdown
	CalculateActualFare(vehicleType string, distanceKm float64, durationMins int, surgeMultiplier float64) *models.FareBreakdown
	CalculateSurge(demandCount, supplyCount int) float64
	EstimateDistance(pickupLat, pickupLng, dropoffLat, dropoffLng float64) float64
	EstimateDuration(distanceKm float64) int
}

type pricingService struct{}

func NewPricingService() PricingService {
	return &pricingService{}
}

func (s *pricingService) CalculateEstimatedFare(vehicleType string, distanceKm float64, durationMins int, surgeMultiplier float64) *models.FareBreakdown {
	return s.calculateFare(vehicleType, distanceKm, durationMins, surgeMultiplier)
}

func (s *pricingService) CalculateActualFare(vehicleType string, distanceKm float64, durationMins int, surgeMultiplier float64) *models.FareBreakdown {
	return s.calculateFare(vehicleType, distanceKm, durationMins, surgeMultiplier)
}

func (s *pricingService) calculateFare(vehicleType string, distanceKm float64, durationMins int, surgeMultiplier float64) *models.FareBreakdown {
	config, exists := fareConfigs[vehicleType]
	if !exists {
		config = fareConfigs[models.VehicleTypeSedan] // default
	}

	baseFare := config.BaseFare
	distanceFare := distanceKm * config.PerKmRate
	timeFare := float64(durationMins) * config.PerMinRate

	subtotal := baseFare + distanceFare + timeFare
	surgeAmount := subtotal * (surgeMultiplier - 1)
	total := subtotal + surgeAmount

	if total < config.MinFare {
		total = config.MinFare
		surgeAmount = 0
	}

	return &models.FareBreakdown{
		BaseFare:     round(baseFare),
		DistanceFare: round(distanceFare),
		TimeFare:     round(timeFare),
		SurgeAmount:  round(surgeAmount),
		Total:        round(total),
	}
}

func (s *pricingService) CalculateSurge(demandCount, supplyCount int) float64 {
	if supplyCount == 0 {
		return 2.0 // Max surge
	}

	ratio := float64(demandCount) / float64(supplyCount)

	switch {
	case ratio < 1.0:
		return 1.0
	case ratio < 1.5:
		return 1.2
	case ratio < 2.0:
		return 1.5
	case ratio < 3.0:
		return 1.8
	default:
		return 2.0
	}
}

// EstimateDistance calculates straight-line distance and multiplies by road factor
func (s *pricingService) EstimateDistance(pickupLat, pickupLng, dropoffLat, dropoffLng float64) float64 {
	straightLine := haversineDistance(pickupLat, pickupLng, dropoffLat, dropoffLng)
	// Multiply by 1.3 to account for actual road distance
	return round(straightLine * 1.3)
}

// EstimateDuration estimates trip duration based on distance (assuming 25 km/h avg speed in city)
func (s *pricingService) EstimateDuration(distanceKm float64) int {
	// Average speed 25 km/h in city traffic
	durationHours := distanceKm / 25.0
	durationMins := int(math.Ceil(durationHours * 60))
	if durationMins < 5 {
		durationMins = 5
	}
	return durationMins
}

// haversineDistance calculates the distance between two points on Earth
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371 // km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func round(f float64) float64 {
	return math.Round(f*100) / 100
}
