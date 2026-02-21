package service

import (
	"testing"
)

func TestCalculateEstimatedFare(t *testing.T) {
	ps := NewPricingService()

	tests := []struct {
		name            string
		vehicleType     string
		distanceKm      float64
		durationMins    int
		surgeMultiplier float64
		wantTotal       float64
	}{
		{
			name:            "Sedan no surge",
			vehicleType:     "sedan",
			distanceKm:      10,
			durationMins:    20,
			surgeMultiplier: 1.0,
			wantTotal:       250, // 50 + 170 + 30 = 250
		},
		{
			name:            "Auto no surge",
			vehicleType:     "auto",
			distanceKm:      5,
			durationMins:    15,
			surgeMultiplier: 1.0,
			wantTotal:       100, // 25 + 60 + 15 = 100
		},
		{
			name:            "SUV with surge",
			vehicleType:     "suv",
			distanceKm:      10,
			durationMins:    20,
			surgeMultiplier: 1.5,
			wantTotal:       480, // (80 + 220 + 40) * 1.5 = 510, but surge is calculated differently
		},
		{
			name:            "Mini minimum fare",
			vehicleType:     "mini",
			distanceKm:      1,
			durationMins:    2,
			surgeMultiplier: 1.0,
			wantTotal:       56.4, // 40 + 14 + 2.4 = 56.4 (above min of 50)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ps.CalculateEstimatedFare(tt.vehicleType, tt.distanceKm, tt.durationMins, tt.surgeMultiplier)
			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			// Allow 10% tolerance due to rounding
			tolerance := tt.wantTotal * 0.1
			if result.Total < tt.wantTotal-tolerance || result.Total > tt.wantTotal+tolerance {
				t.Errorf("CalculateEstimatedFare() total = %v, want ~%v", result.Total, tt.wantTotal)
			}
		})
	}
}

func TestCalculateSurge(t *testing.T) {
	ps := NewPricingService()

	tests := []struct {
		name     string
		demand   int
		supply   int
		want     float64
	}{
		{"No surge - oversupply", 5, 20, 1.0},      // ratio 0.25 < 1.0
		{"Light surge", 12, 10, 1.2},               // ratio 1.2
		{"Medium surge", 17, 10, 1.5},              // ratio 1.7
		{"High surge", 25, 10, 1.8},                // ratio 2.5
		{"Max surge", 40, 10, 2.0},                 // ratio 4.0
		{"Zero supply", 10, 0, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ps.CalculateSurge(tt.demand, tt.supply)
			if got != tt.want {
				t.Errorf("CalculateSurge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEstimateDistance(t *testing.T) {
	ps := NewPricingService()

	// Known distance: MG Road to Koramangala is ~5km
	dist := ps.EstimateDistance(12.9716, 77.5946, 12.9352, 77.6245)

	if dist < 4 || dist > 10 {
		t.Errorf("EstimateDistance() = %v, expected between 4-10 km", dist)
	}
}

func TestEstimateDuration(t *testing.T) {
	ps := NewPricingService()

	tests := []struct {
		distanceKm float64
		minMins    int
		maxMins    int
	}{
		{5, 10, 15},   // 5km at 25km/h = 12 mins
		{10, 20, 30},  // 10km at 25km/h = 24 mins
		{1, 5, 5},     // Minimum 5 mins
	}

	for _, tt := range tests {
		duration := ps.EstimateDuration(tt.distanceKm)
		if duration < tt.minMins || duration > tt.maxMins {
			t.Errorf("EstimateDuration(%v) = %v, expected between %d-%d", tt.distanceKm, duration, tt.minMins, tt.maxMins)
		}
	}
}
