package service

import (
	"testing"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

func params(sf, tx int, weather float64) repository.SessionParams {
	return repository.SessionParams{SpreadingFactor: sf, TxPowerDbm: tx, WeatherSeverity: weather}
}

func TestEvaluateDeterministic(t *testing.T) {
	// Roll 0.0 -> 0 on the 0..100 scale: any loss probability > 0 drops.
	alwaysLow := NewEnvironmentalAnomalyServiceWithRand(func() float64 { return 0.0 })
	v := alwaysLow.Evaluate(1.0, params(7, 20, 1.0)) // 5% loss
	if !v.Dropped {
		t.Error("roll 0 against 5%% loss must drop")
	}

	// Roll 0.999 -> 99.9: survives anything except a 100% loss.
	alwaysHigh := NewEnvironmentalAnomalyServiceWithRand(func() float64 { return 0.999 })
	v = alwaysHigh.Evaluate(1.0, params(7, 20, 1.0))
	if v.Dropped {
		t.Error("roll 99.9 against 5%% loss must survive")
	}

	// Beyond radio range: 100% loss regardless of the roll.
	v = alwaysHigh.Evaluate(50.0, params(7, 20, 1.0)) // range at 20 dBm = 10 km
	if !v.Dropped || !v.OutOfRange {
		t.Errorf("50 km at 20 dBm must be an out-of-range drop, got %+v", v)
	}
}

func TestEvaluateWeatherMultiplier(t *testing.T) {
	svc := NewEnvironmentalAnomalyServiceWithRand(func() float64 { return 0.5 })

	calm := svc.Evaluate(3.0, params(7, 20, 1.0)) // base 20%
	storm := svc.Evaluate(3.0, params(7, 20, 1.5))
	if calm.LossPercent != 20 {
		t.Errorf("calm 3 km loss = %f, want 20", calm.LossPercent)
	}
	if storm.LossPercent != 30 {
		t.Errorf("storm 3 km loss = %f, want 30 (20%% × 1.5)", storm.LossPercent)
	}
}
