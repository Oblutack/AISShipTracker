package ais

import (
	"math"
	"testing"
)

func almostEqual(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestGreatCirclePath_EndpointsExact(t *testing.T) {
	path := GreatCirclePath(51.9244, 4.4777, 40.6892, -74.0445, 20) // Rotterdam -> New York
	if len(path) != 20 {
		t.Fatalf("len(path) = %d, want 20", len(path))
	}
	if !almostEqual(path[0][0], 51.9244, 1e-6) || !almostEqual(path[0][1], 4.4777, 1e-6) {
		t.Errorf("first point = %v, want (51.9244, 4.4777)", path[0])
	}
	last := path[len(path)-1]
	if !almostEqual(last[0], 40.6892, 1e-6) || !almostEqual(last[1], -74.0445, 1e-6) {
		t.Errorf("last point = %v, want (40.6892, -74.0445)", last)
	}
}

func TestGreatCirclePath_EquatorStaysOnEquator(t *testing.T) {
	// The equator is itself a great circle, so every interpolated point
	// between two equatorial points must also have latitude ~0.
	path := GreatCirclePath(0, -50, 0, 50, 10)
	for i, p := range path {
		if !almostEqual(p[0], 0, 1e-6) {
			t.Errorf("point %d latitude = %v, want ~0", i, p[0])
		}
	}
}

func TestGreatCirclePath_IdenticalPointsNoDivideByZero(t *testing.T) {
	path := GreatCirclePath(10, 10, 10, 10, 5)
	for i, p := range path {
		if !almostEqual(p[0], 10, 1e-9) || !almostEqual(p[1], 10, 1e-9) {
			t.Errorf("point %d = %v, want (10,10)", i, p)
		}
	}
}

func TestGreatCirclePath_MinimumTwoPoints(t *testing.T) {
	path := GreatCirclePath(0, 0, 10, 10, 1)
	if len(path) != 2 {
		t.Errorf("len(path) = %d, want 2 (clamped up from 1)", len(path))
	}
}
