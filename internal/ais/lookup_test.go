package ais

import "testing"

func TestFlag(t *testing.T) {
	cases := []struct {
		mmsi int
		want string
	}{
		{257123456, "Norway"},
		{636012345, "Liberia"},
		{100000000, "Unknown flag"}, // out of valid MMSI range
		{999999999, "Unknown flag"}, // valid range, unmapped MID
	}
	for _, c := range cases {
		if got := Flag(c.mmsi); got != c.want {
			t.Errorf("Flag(%d) = %q, want %q", c.mmsi, got, c.want)
		}
	}
}

func TestTypeLabel(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{70, "Cargo"},
		{80, "Tanker"},
		{60, "Passenger"},
		{75, "Cargo"}, // reserved sub-code, falls back to group label
		{999, "Unknown"},
	}
	for _, c := range cases {
		if got := TypeLabel(c.code); got != c.want {
			t.Errorf("TypeLabel(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestIsCargoCarrying(t *testing.T) {
	if !IsCargoCarrying(70) || !IsCargoCarrying(89) {
		t.Error("cargo/tanker codes (70-89) should be cargo-carrying")
	}
	if IsCargoCarrying(60) || IsCargoCarrying(37) {
		t.Error("passenger/pleasure craft codes should not be cargo-carrying")
	}
}
