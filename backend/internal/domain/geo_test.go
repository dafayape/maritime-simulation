package domain

import (
	"math"
	"testing"
)

func TestHaversineZero(t *testing.T) {
	if d := HaversineKm(-6.98, 106.55, -6.98, 106.55); d != 0 {
		t.Errorf("identical points must be 0 km apart, got %f", d)
	}
}

func TestHaversineOneDegree(t *testing.T) {
	// One degree of latitude ≈ 111.19 km on the IUGG sphere.
	d := HaversineKm(0, 0, 1, 0)
	if math.Abs(d-111.19) > 0.3 {
		t.Errorf("1° latitude = %f km, want ≈111.19", d)
	}
	// One degree of longitude at the equator is the same.
	d = HaversineKm(0, 0, 0, 1)
	if math.Abs(d-111.19) > 0.3 {
		t.Errorf("1° longitude at equator = %f km, want ≈111.19", d)
	}
}

func TestHaversineSymmetry(t *testing.T) {
	a := HaversineKm(-6.9875, 106.5504, -7.1853, 106.4521)
	b := HaversineKm(-7.1853, 106.4521, -6.9875, 106.5504)
	if math.Abs(a-b) > 1e-9 {
		t.Errorf("distance must be symmetric: %f vs %f", a, b)
	}
	if a < 20 || a > 30 {
		t.Errorf("Pelabuhan Ratu to offshore point = %f km, expected ~24 km", a)
	}
}
