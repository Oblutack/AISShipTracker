package store

import (
	"testing"
	"time"

	"github.com/Oblutack/AISShipTracker/internal/ais"
)

func TestApplyPosition_ThenStatic_Merges(t *testing.T) {
	s := New()

	s.ApplyPosition(ais.PositionUpdate{
		MMSI: 257123456, Latitude: 60.1, Longitude: 5.2, Sog: 10, ReceivedAt: time.Now(),
	})
	v, ok := s.Get(257123456)
	if !ok {
		t.Fatal("expected vessel to exist after position update")
	}
	if !v.HasPosition || v.HasStatic {
		t.Errorf("HasPosition=%v HasStatic=%v, want true/false", v.HasPosition, v.HasStatic)
	}
	if v.Flag != "Norway" {
		t.Errorf("Flag = %q, want Norway (derived from MMSI on first creation)", v.Flag)
	}

	s.ApplyStatic(ais.StaticUpdate{
		MMSI: 257123456, Name: "TEST VESSEL", ShipType: 70, ReceivedAt: time.Now(),
	})
	v, _ = s.Get(257123456)
	if !v.HasPosition || !v.HasStatic {
		t.Errorf("HasPosition=%v HasStatic=%v, want true/true after both updates", v.HasPosition, v.HasStatic)
	}
	// Position fields from the first update must survive the static merge.
	if v.Latitude != 60.1 {
		t.Errorf("Latitude = %v, want 60.1 (should not be clobbered by static update)", v.Latitude)
	}
	if v.Name != "TEST VESSEL" || v.TypeLabel != "Cargo" {
		t.Errorf("Name/TypeLabel = %q/%q, want TEST VESSEL/Cargo", v.Name, v.TypeLabel)
	}
}

func TestPrune_RemovesOnlyStale(t *testing.T) {
	s := New()
	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, ReceivedAt: time.Now().Add(-time.Hour)})
	s.ApplyPosition(ais.PositionUpdate{MMSI: 2, ReceivedAt: time.Now()})

	removed := s.Prune(30 * time.Minute)
	if removed != 1 {
		t.Errorf("Prune removed %d, want 1", removed)
	}
	if _, ok := s.Get(1); ok {
		t.Error("stale vessel 1 should have been pruned")
	}
	if _, ok := s.Get(2); !ok {
		t.Error("fresh vessel 2 should still be present")
	}
}

func TestList_ReturnsAllVessels(t *testing.T) {
	s := New()
	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, ReceivedAt: time.Now()})
	s.ApplyPosition(ais.PositionUpdate{MMSI: 2, ReceivedAt: time.Now()})
	if got := len(s.List()); got != 2 {
		t.Errorf("List() returned %d vessels, want 2", got)
	}
}
