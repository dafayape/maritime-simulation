package domain

// Artificial Packet Loss model (SRS §5B). The backend rolls these dice for
// every radio traversal — data frames and returning ACK signals alike.

// BaseLossPercent maps sender→receiver air distance to the base probability
// (in percent) that the frame dies mid-air:
//
//	D <= 2 km       ->  5%
//	2 km < D <= 5km -> 20%
//	D > 5 km        -> 60%
func BaseLossPercent(distanceKm float64) float64 {
	switch {
	case distanceKm <= 2.0:
		return 5.0
	case distanceKm <= 5.0:
		return 20.0
	default:
		return 60.0
	}
}

// FinalLossPercent applies the out-of-range cutoff and the weather multiplier:
//
//	P_f = min(100, P_b × weatherSeverity), or 100% when D exceeds radio range.
func FinalLossPercent(distanceKm, maxRangeKm, weatherSeverity float64) float64 {
	if distanceKm > maxRangeKm {
		return 100.0
	}
	p := BaseLossPercent(distanceKm) * weatherSeverity
	if p > 100.0 {
		return 100.0
	}
	if p < 0 {
		return 0
	}
	return p
}

// RollDrop decides a single traversal. roll must be uniform in [0,100);
// injecting it (instead of calling math/rand here) keeps the model
// deterministic under test.
func RollDrop(roll, finalLossPercent float64) bool {
	return roll < finalLossPercent
}
