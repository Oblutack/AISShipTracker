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

func TestTrack_ThrottlesRapidUpdates(t *testing.T) {
	s := New()
	base := time.Now()

	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, Latitude: 1, Longitude: 1, ReceivedAt: base})
	// Well within trackMinInterval - should NOT add a second point.
	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, Latitude: 1.001, Longitude: 1, ReceivedAt: base.Add(5 * time.Second)})
	if got := len(s.Track(1)); got != 1 {
		t.Errorf("Track length = %d, want 1 (second update within throttle window)", got)
	}

	// Past trackMinInterval - should add a second point.
	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, Latitude: 1.002, Longitude: 1, ReceivedAt: base.Add(45 * time.Second)})
	if got := len(s.Track(1)); got != 2 {
		t.Errorf("Track length = %d, want 2 (update past throttle window)", got)
	}
}

func TestTrack_CapsAtMaxPoints(t *testing.T) {
	s := New()
	base := time.Now()
	for i := 0; i < trackMaxPoints+20; i++ {
		s.ApplyPosition(ais.PositionUpdate{
			MMSI: 1, Latitude: float64(i), Longitude: 0,
			ReceivedAt: base.Add(time.Duration(i) * trackMinInterval),
		})
	}
	track := s.Track(1)
	if len(track) != trackMaxPoints {
		t.Fatalf("Track length = %d, want %d (capped)", len(track), trackMaxPoints)
	}
	// Oldest points should have been dropped - the first remaining point
	// should NOT be latitude 0 anymore.
	if track[0].Latitude == 0 {
		t.Error("expected oldest track points to have been evicted, but latitude 0 (the first-ever point) is still present")
	}
	// The most recent point must be the last one applied.
	last := track[len(track)-1]
	if last.Latitude != float64(trackMaxPoints+19) {
		t.Errorf("last track point latitude = %v, want %v", last.Latitude, float64(trackMaxPoints+19))
	}
}

func TestPrune_AlsoRemovesTrackHistory(t *testing.T) {
	s := New()
	s.ApplyPosition(ais.PositionUpdate{MMSI: 1, ReceivedAt: time.Now().Add(-time.Hour)})
	s.Prune(30 * time.Minute)
	if got := len(s.Track(1)); got != 0 {
		t.Errorf("Track(1) after prune = %d points, want 0 (orphaned history should be cleaned up)", got)
	}
}
