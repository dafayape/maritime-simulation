package domain

import (
	"fmt"
	"math"
	"time"
)

// LoRa physical-layer invariants (SRS §5A + PRD domain rules).
const (
	MinSpreadingFactor = 7
	MaxSpreadingFactor = 12

	MinTxPowerDbm = 2  // SX1276 practical minimum
	MaxTxPowerDbm = 20 // SX1276 PA_BOOST maximum

	// MaxRetries is the Data Link auto-retry budget: after 3 failed
	// retransmissions a node must declare the link dead and wait for a new
	// route from MeshTopologyService.
	MaxRetries = 3

	// MinWeatherSeverity / MaxWeatherSeverity bound the packet-loss
	// multiplier. 1.0 = calm sea (baseline), 2.0 = full storm.
	MinWeatherSeverity = 0.0
	MaxWeatherSeverity = 2.0
)

// MaxPayloadBytes returns the hard payload ceiling for a Spreading Factor.
// Violations are dropped without ACK, exactly like an SX1276 refusing to
// modulate an oversized frame.
//
//	SF 7-8  -> 242 bytes
//	SF 9    -> 115 bytes
//	SF 10-12 -> 51 bytes
func MaxPayloadBytes(sf int) int {
	switch {
	case sf <= 8:
		return 242
	case sf == 9:
		return 115
	default:
		return 51
	}
}

// ackTimeouts approximates LoRa time-on-air growth: every SF step roughly
// doubles airtime, so the ACK wait window grows with SF (PRD: "SF tinggi =
// Timeout lebih lama").
var ackTimeouts = map[int]time.Duration{
	7:  1000 * time.Millisecond,
	8:  1500 * time.Millisecond,
	9:  2500 * time.Millisecond,
	10: 4000 * time.Millisecond,
	11: 6500 * time.Millisecond,
	12: 10000 * time.Millisecond,
}

// AckTimeout returns how long a sender waits for an ACK before retrying.
func AckTimeout(sf int) time.Duration {
	if d, ok := ackTimeouts[sf]; ok {
		return d
	}
	if sf < MinSpreadingFactor {
		return ackTimeouts[MinSpreadingFactor]
	}
	return ackTimeouts[MaxSpreadingFactor]
}

// MaxRangeKm models radio reach from TX power using the free-space link
// budget rule of thumb: +6 dB doubles the distance. Anchored at
// 14 dBm ≈ 5 km over open water, so 20 dBm ≈ 10 km.
func MaxRangeKm(txPowerDbm int) float64 {
	return 5.0 * math.Pow(2, float64(txPowerDbm-14)/6.0)
}

// ValidateSpreadingFactor rejects SF outside 7..12.
func ValidateSpreadingFactor(sf int) error {
	if sf < MinSpreadingFactor || sf > MaxSpreadingFactor {
		return fmt.Errorf("%w: spreading_factor must be between %d and %d, got %d",
			ErrValidation, MinSpreadingFactor, MaxSpreadingFactor, sf)
	}
	return nil
}

// ValidateTxPower rejects TX power outside the SX1276 envelope.
func ValidateTxPower(dbm int) error {
	if dbm < MinTxPowerDbm || dbm > MaxTxPowerDbm {
		return fmt.Errorf("%w: tx_power_dbm must be between %d and %d, got %d",
			ErrValidation, MinTxPowerDbm, MaxTxPowerDbm, dbm)
	}
	return nil
}

// ValidateWeatherSeverity rejects multipliers outside 0.0..2.0.
func ValidateWeatherSeverity(ws float64) error {
	if ws < MinWeatherSeverity || ws > MaxWeatherSeverity {
		return fmt.Errorf("%w: weather_severity must be between %.1f and %.1f, got %.2f",
			ErrValidation, MinWeatherSeverity, MaxWeatherSeverity, ws)
	}
	return nil
}
