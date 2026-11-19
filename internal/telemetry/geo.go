// Package telemetry derives motion intelligence (heading, distance travelled,
// staleness) from the raw GPS fixes Chalo reports, and keeps a bounded
// per-bus history of recent positions.
package telemetry

import "math"

const earthRadiusMeters = 6371008.8 // IUGG mean Earth radius

func toRad(deg float64) float64 { return deg * math.Pi / 180 }
func toDeg(rad float64) float64 { return rad * 180 / math.Pi }

// HaversineMeters returns the great-circle distance between two points.
func HaversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	φ1, φ2 := toRad(lat1), toRad(lat2)
	dφ := toRad(lat2 - lat1)
	dλ := toRad(lon2 - lon1)
	a := math.Sin(dφ/2)*math.Sin(dφ/2) + math.Cos(φ1)*math.Cos(φ2)*math.Sin(dλ/2)*math.Sin(dλ/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// BearingDegrees returns the initial great-circle bearing from point 1 to
// point 2, in degrees clockwise from true north, normalized to [0, 360).
func BearingDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	φ1, φ2 := toRad(lat1), toRad(lat2)
	dλ := toRad(lon2 - lon1)
	y := math.Sin(dλ) * math.Cos(φ2)
	x := math.Cos(φ1)*math.Sin(φ2) - math.Sin(φ1)*math.Cos(φ2)*math.Cos(dλ)
	b := math.Mod(toDeg(math.Atan2(y, x))+360, 360)
	// Guard against -0 and floating error yielding exactly 360.
	if b >= 360 || b < 0 {
		b = 0
	}
	return b
}
