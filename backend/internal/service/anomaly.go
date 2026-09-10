package service

import (
	"math/rand/v2"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

// EnvironmentalAnomalyService is the probabilistic chaos injector (PRD §3.2):
// every radio traversal — data frame or returning ACK — rolls its dice here.
// The random source is injectable so the engine's behaviour is deterministic
// under test.
type EnvironmentalAnomalyService struct {
	randFloat01 func() float64
}

// NewEnvironmentalAnomalyService uses math/rand/v2 (simulation quality, not
// cryptographic — deliberately).
func NewEnvironmentalAnomalyService() *EnvironmentalAnomalyService {
	return &EnvironmentalAnomalyService{randFloat01: rand.Float64}
}

// NewEnvironmentalAnomalyServiceWithRand injects a deterministic source.
func NewEnvironmentalAnomalyServiceWithRand(randFloat01 func() float64) *EnvironmentalAnomalyService {
	return &EnvironmentalAnomalyService{randFloat01: randFloat01}
}

// TraversalVerdict is the outcome of one simulated radio traversal.
type TraversalVerdict struct {
	Dropped     bool
	OutOfRange  bool
	DistanceKm  float64
	LossPercent float64
}

// Evaluate applies SRS §5B: base loss from distance, out-of-range cutoff
// from TX power, weather multiplier, then a uniform roll in [0,100).
func (s *EnvironmentalAnomalyService) Evaluate(distanceKm float64, params repository.SessionParams) TraversalVerdict {
	maxRange := domain.MaxRangeKm(params.TxPowerDbm)
	loss := domain.FinalLossPercent(distanceKm, maxRange, params.WeatherSeverity)
	roll := s.randFloat01() * 100.0
	return TraversalVerdict{
		Dropped:     domain.RollDrop(roll, loss),
		OutOfRange:  distanceKm > maxRange,
		DistanceKm:  distanceKm,
		LossPercent: loss,
	}
}
