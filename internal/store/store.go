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

// Store is a thread-safe, in-memory cache of the latest known state per MMSI.
type Store struct {
	mu      sync.RWMutex
	vessels map[int]*ais.Vessel
}

// New creates an empty Store.
func New() *Store {
	return &Store{
		vessels: make(map[int]*ais.Vessel),
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

	return *v
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
	v.ShipType = u.ShipType
	v.TypeLabel = ais.TypeLabel(u.ShipType)
	v.Destination = u.Destination
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
			removed++
		}
	}
	return removed
}
