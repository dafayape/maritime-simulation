package domain

import "math"

// earthRadiusKm is the IUGG mean Earth radius.
const earthRadiusKm = 6371.0088

// HaversineKm computes the great-circle distance between two coordinates in
// kilometres. This is the same "air distance" the anomaly service uses to
// price packet loss (SRS §5B step 1).
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}
