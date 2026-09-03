package ais

import "math"

// GreatCirclePath returns `points` coordinates tracing the great-circle
// (shortest-on-a-sphere) arc from (lat1,lon1) to (lat2,lon2), inclusive of
// both endpoints. This is a geometric approximation only - it treats Earth
// as a sphere and has no notion of coastlines, shipping lanes, or land at
// all. It will happily draw a line straight across a continent if that's
// where the great circle falls. Callers must present this as an inferred
// straight-line estimate, not a real route - see ais.InferredRouteNotice.
func GreatCirclePath(lat1, lon1, lat2, lon2 float64, points int) [][2]float64 {
	if points < 2 {
		points = 2
	}

	phi1, lam1 := deg2rad(lat1), deg2rad(lon1)
	phi2, lam2 := deg2rad(lat2), deg2rad(lon2)

	// Central angle between the two points (haversine form).
	dPhi := phi2 - phi1
	dLam := lam2 - lam1
	a := math.Pow(math.Sin(dPhi/2), 2) + math.Cos(phi1)*math.Cos(phi2)*math.Pow(math.Sin(dLam/2), 2)
	d := 2 * math.Asin(math.Min(1, math.Sqrt(a)))

	out := make([][2]float64, points)

	if d < 1e-12 {
		// Identical (or effectively identical) endpoints - nothing to
		// interpolate, every point is just that point.
		for i := range out {
			out[i] = [2]float64{lat1, lon1}
		}
		return out
	}

	for i := 0; i < points; i++ {
		f := float64(i) / float64(points-1)
		A := math.Sin((1-f)*d) / math.Sin(d)
		B := math.Sin(f*d) / math.Sin(d)

		x := A*math.Cos(phi1)*math.Cos(lam1) + B*math.Cos(phi2)*math.Cos(lam2)
		y := A*math.Cos(phi1)*math.Sin(lam1) + B*math.Cos(phi2)*math.Sin(lam2)
		z := A*math.Sin(phi1) + B*math.Sin(phi2)

		lat := math.Atan2(z, math.Sqrt(x*x+y*y))
		lon := math.Atan2(y, x)

		out[i] = [2]float64{rad2deg(lat), rad2deg(lon)}
	}

	return out
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }
func rad2deg(r float64) float64 { return r * 180 / math.Pi }

// InferredRouteNotice is the label the UI should attach to anything drawn
// from GreatCirclePath, so it's never mistaken for a real reported path.
const InferredRouteNotice = "Inferred: straight-line great-circle estimate to a best-effort matched destination port - not a real route, not ocean-routed, not reported by the vessel."
