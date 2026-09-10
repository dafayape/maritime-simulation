package domain

import "testing"

func TestBaseLossPercentTiers(t *testing.T) {
	cases := []struct {
		distKm float64
		want   float64
	}{
		{0.1, 5}, {2.0, 5}, {2.01, 20}, {5.0, 20}, {5.01, 60}, {50, 60},
	}
	for _, c := range cases {
		if got := BaseLossPercent(c.distKm); got != c.want {
			t.Errorf("BaseLossPercent(%f) = %f, want %f", c.distKm, got, c.want)
		}
	}
}

func TestFinalLossPercent(t *testing.T) {
	// Calm sea, short hop: base 5% × 1.0.
	if got := FinalLossPercent(1.0, 10, 1.0); got != 5 {
		t.Errorf("calm short hop = %f, want 5", got)
	}
	// Storm multiplies: 20% × 1.5 = 30%.
	if got := FinalLossPercent(3.0, 10, 1.5); got != 30 {
		t.Errorf("storm mid hop = %f, want 30", got)
	}
	// Beyond radio range is always 100%.
	if got := FinalLossPercent(11.0, 10, 1.0); got != 100 {
		t.Errorf("out of range = %f, want 100", got)
	}
	// Cap at 100: 60% × 2.0 = 120 → 100.
	if got := FinalLossPercent(6.0, 10, 2.0); got != 100 {
		t.Errorf("capped storm = %f, want 100", got)
	}
}

func TestRollDrop(t *testing.T) {
	if RollDrop(50, 0) {
		t.Error("0%% loss must never drop")
	}
	if !RollDrop(99.99, 100) {
		t.Error("100%% loss must always drop")
	}
	if !RollDrop(4.9, 5) {
		t.Error("roll below threshold must drop")
	}
	if RollDrop(5.0, 5) {
		t.Error("roll at threshold must survive")
	}
}
