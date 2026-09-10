package domain

import (
	"math"
	"testing"
)

func TestMaxPayloadBytes(t *testing.T) {
	cases := map[int]int{7: 242, 8: 242, 9: 115, 10: 51, 11: 51, 12: 51}
	for sf, want := range cases {
		if got := MaxPayloadBytes(sf); got != want {
			t.Errorf("MaxPayloadBytes(%d) = %d, want %d", sf, got, want)
		}
	}
}

func TestAckTimeoutGrowsWithSF(t *testing.T) {
	prev := AckTimeout(MinSpreadingFactor)
	for sf := MinSpreadingFactor + 1; sf <= MaxSpreadingFactor; sf++ {
		cur := AckTimeout(sf)
		if cur <= prev {
			t.Errorf("AckTimeout(%d)=%v is not greater than AckTimeout(%d)=%v", sf, cur, sf-1, prev)
		}
		prev = cur
	}
}

func TestMaxRangeKmAnchors(t *testing.T) {
	if got := MaxRangeKm(14); math.Abs(got-5.0) > 1e-9 {
		t.Errorf("MaxRangeKm(14) = %f, want 5.0", got)
	}
	if got := MaxRangeKm(20); math.Abs(got-10.0) > 1e-9 {
		t.Errorf("MaxRangeKm(20) = %f, want 10.0 (+6dB doubles range)", got)
	}
	if MaxRangeKm(8) >= MaxRangeKm(14) {
		t.Error("lower TX power must yield shorter range")
	}
}

func TestValidators(t *testing.T) {
	if err := ValidateSpreadingFactor(7); err != nil {
		t.Errorf("SF 7 should be valid: %v", err)
	}
	if err := ValidateSpreadingFactor(6); err == nil {
		t.Error("SF 6 should be rejected")
	}
	if err := ValidateSpreadingFactor(13); err == nil {
		t.Error("SF 13 should be rejected")
	}
	if err := ValidateTxPower(20); err != nil {
		t.Errorf("TX 20 should be valid: %v", err)
	}
	if err := ValidateTxPower(25); err == nil {
		t.Error("TX 25 should be rejected")
	}
	if err := ValidateWeatherSeverity(1.8); err != nil {
		t.Errorf("weather 1.8 should be valid: %v", err)
	}
	if err := ValidateWeatherSeverity(2.5); err == nil {
		t.Error("weather 2.5 should be rejected")
	}
	if err := ValidateWeatherSeverity(-0.1); err == nil {
		t.Error("weather -0.1 should be rejected")
	}
}
