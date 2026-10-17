// Package store holds the current-state cache of every vessel seen so far.
//
// It is deliberately in-memory only for this first version: a live tracker
// mostly cares about "where is this ship right now", and an in-memory map
// keeps the whole pipeline runnable with nothing but a feed connection - no
// database to stand up before you see your first real ship. The Store type
// is a small, self-contained interface-shaped struct on purpose: swapping
// in Postgres/PostGIS persistence later (for historical track playback,
// durability across restarts) means adding a second implementation with the
// same method set, not rewriting the callers.
package store

import (
	"sync"
	"time"

	"github.com/Oblutack/AISShipTracker/internal/ais"
)

// Position history is deliberately sparse and bounded: a new point is only
// recorded every trackMinInterval, and only the most recent trackMaxPoints
// are kept per vessel. A ship broadcasts every few seconds - recording
// every single report would be both wasteful (10,000+ vessels, unbounded
// growth) and pointless (a trail this dense adds nothing visually over a
// coarser one).
const (
	trackMinInterval = 30 * time.Second
	trackMaxPoints   = 120 // ~1 hour of trail at the interval above
)

// Store is a thread-safe, in-memory cache of the latest known state per MMSI.
type Store struct {
	mu      sync.RWMutex
	vessels map[int]*ais.Vessel
	tracks  map[int][]ais.TrackPoint
}

// New creates an empty Store.
func New() *Store {
	return &Store{
		vessels: make(map[int]*ais.Vessel),
		tracks:  make(map[int][]ais.TrackPoint),
	}
}

// ApplyPosition merges a position report into the vessel record for its
// MMSI, creating the record if this is the first message ever seen for it.
// Returns the full, updated vessel so the caller can broadcast it.
func (s *Store) ApplyPosition(u ais.PositionUpdate) ais.Vessel {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.getOrCreate(u.MMSI)
	v.Latitude = u.Latitude
	v.Longitude = u.Longitude
	v.Cog = u.Cog
	v.Sog = u.Sog
	v.Heading = u.Heading
	v.NavStatus = u.NavStatus
	v.NavStatusLabel = ais.NavStatusLabel(u.NavStatus)
	v.HasPosition = true
	v.LastUpdate = u.ReceivedAt

	s.recordTrackPoint(u.MMSI, u.Latitude, u.Longitude, u.ReceivedAt)

	return *v
}

// recordTrackPoint must be called with s.mu held. It appends a new track
// point only if enough time has passed since the last recorded one, and
// caps the stored history at trackMaxPoints (oldest dropped first).
func (s *Store) recordTrackPoint(mmsi int, lat, lon float64, at time.Time) {
	pts := s.tracks[mmsi]
	if len(pts) > 0 && at.Sub(pts[len(pts)-1].Time) < trackMinInterval {
		return
	}
	pts = append(pts, ais.TrackPoint{Latitude: lat, Longitude: lon, Time: at})
	if len(pts) > trackMaxPoints {
		pts = pts[len(pts)-trackMaxPoints:]
	}
	s.tracks[mmsi] = pts
}

// Track returns the recorded position history for one MMSI, oldest first.
func (s *Store) Track(mmsi int) []ais.TrackPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pts := s.tracks[mmsi]
	out := make([]ais.TrackPoint, len(pts))
	copy(out, pts)
	return out
}

// ApplyStatic merges a static/voyage data report into the vessel record for
// its MMSI, creating the record if needed. Returns the full, updated vessel.
func (s *Store) ApplyStatic(u ais.StaticUpdate) ais.Vessel {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := s.getOrCreate(u.MMSI)
	if u.IMO != 0 {
		v.IMO = u.IMO
	}
	if u.Name != "" {
		v.Name = u.Name
	}
	if u.CallSign != "" {
		v.CallSign = u.CallSign
	}
	// ShipType and Destination used to be set unconditionally, which was
	// fine while every static update came from a single ShipStaticData
	// message carrying all fields at once. It stopped being fine once
	// StaticDataReport (AIS type 24, Class B) started feeding in here too:
	// that message type arrives as two separate halves per MMSI (Part A:
	// name only, Part B: type/callsign/dimensions only, neither carries
	// Destination at all), so a lone Part A update would otherwise zero
	// out a ShipType/Destination already learned from an earlier message.
	// 0/"" is AIS's own "not available" sentinel for both fields anyway,
	// so skipping the write when empty loses nothing real.
	if u.ShipType != 0 {
		v.ShipType = u.ShipType
		v.TypeLabel = ais.TypeLabel(u.ShipType)
	}
	if u.Destination != "" {
		v.Destination = u.Destination
	}
	if u.LengthMeters != 0 {
		v.LengthMeters = u.LengthMeters
	}
	if u.BeamMeters != 0 {
		v.BeamMeters = u.BeamMeters
	}
	if u.DraughtMeters != 0 {
		v.DraughtMeters = u.DraughtMeters
	}
	v.HasStatic = true
	v.LastUpdate = u.ReceivedAt

	return *v
}

// getOrCreate must be called with s.mu held.
func (s *Store) getOrCreate(mmsi int) *ais.Vessel {
	v, ok := s.vessels[mmsi]
	if !ok {
		v = &ais.Vessel{
			MMSI: mmsi,
			Flag: ais.Flag(mmsi),
		}
		s.vessels[mmsi] = v
	}
	return v
}

// Get returns the current record for one MMSI.
func (s *Store) Get(mmsi int) (ais.Vessel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vessels[mmsi]
	if !ok {
		return ais.Vessel{}, false
	}
	return *v, true
}

// List returns a snapshot of every vessel currently in the store.
func (s *Store) List() []ais.Vessel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ais.Vessel, 0, len(s.vessels))
	for _, v := range s.vessels {
		out = append(out, *v)
	}
	return out
}

// Prune removes vessels whose last update is older than maxAge. AIS
// coverage gaps and decommissioned/renamed MMSIs would otherwise let the
// store grow forever; call this periodically from a ticker.
func (s *Store) Prune(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for mmsi, v := range s.vessels {
		if v.LastUpdate.Before(cutoff) {
			delete(s.vessels, mmsi)
			delete(s.tracks, mmsi)
			removed++
		}
	}
	return removed
}
