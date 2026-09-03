package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Oblutack/AISShipTracker/internal/ais"
	"github.com/Oblutack/AISShipTracker/internal/hub"
	"github.com/Oblutack/AISShipTracker/internal/store"
)

func newTestApp() (*store.Store, func(method, path string) map[string]any) {
	st := store.New()
	h := hub.New()
	go h.Run()
	app := New(st, h)

	do := func(method, path string) map[string]any {
		req := httptest.NewRequest(method, path, nil)
		resp, err := app.Test(req)
		if err != nil {
			panic(err)
		}
		var out map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			panic(err)
		}
		return out
	}
	return st, do
}

// Regression test: a vessel whose only message so far is static/voyage
// data (no position report yet) is stored with Latitude/Longitude at their
// zero value (0,0). /route must refuse to compute anything from that
// rather than silently drawing a line from "null island" - this exact bug
// was caught live against real aisstream.io traffic before this test
// existed.
func TestRouteEndpoint_RefusesVesselWithNoPosition(t *testing.T) {
	st, do := newTestApp()
	st.ApplyStatic(ais.StaticUpdate{MMSI: 123, Destination: "ROTTERDAM", ReceivedAt: time.Now()})

	out := do("GET", "/api/vessels/123/route")
	if matched, _ := out["matched"].(bool); matched {
		t.Fatalf("expected matched=false for a vessel with no position yet, got %v", out)
	}
}

func TestRouteEndpoint_MatchesWhenPositionAndDestinationKnown(t *testing.T) {
	st, do := newTestApp()
	st.ApplyPosition(ais.PositionUpdate{MMSI: 123, Latitude: 51.9, Longitude: 4.5, ReceivedAt: time.Now()})
	st.ApplyStatic(ais.StaticUpdate{MMSI: 123, Destination: "ROTTERDAM", ReceivedAt: time.Now()})

	out := do("GET", "/api/vessels/123/route")
	matched, _ := out["matched"].(bool)
	if !matched {
		t.Fatalf("expected matched=true, got %v", out)
	}
	path, ok := out["path"].([]any)
	if !ok || len(path) < 2 {
		t.Errorf("expected a multi-point path, got %v", out["path"])
	}
}

func TestRouteEndpoint_UnknownVessel404s(t *testing.T) {
	_, do := newTestApp()
	out := do("GET", "/api/vessels/999999/route")
	if _, hasError := out["error"]; !hasError {
		t.Errorf("expected an error field for an unknown vessel, got %v", out)
	}
}

func TestTrackEndpoint_ReturnsRecordedPoints(t *testing.T) {
	st, _ := newTestApp()
	now := time.Now()
	st.ApplyPosition(ais.PositionUpdate{MMSI: 123, Latitude: 1, Longitude: 1, ReceivedAt: now})
	st.ApplyPosition(ais.PositionUpdate{MMSI: 123, Latitude: 2, Longitude: 2, ReceivedAt: now.Add(time.Minute)})

	req := httptest.NewRequest("GET", "/api/vessels/123/track", nil)
	resp, err := New(st, hub.New()).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var track []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		t.Fatal(err)
	}
	if len(track) != 2 {
		t.Errorf("track length = %d, want 2", len(track))
	}
}

func TestPortsEndpoint_NoQueryReturnsFullDataset(t *testing.T) {
	st, _ := newTestApp()
	req := httptest.NewRequest("GET", "/api/ports", nil)
	resp, err := New(st, hub.New()).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var ports []ais.Port
	if err := json.NewDecoder(resp.Body).Decode(&ports); err != nil {
		t.Fatal(err)
	}
	if len(ports) != len(ais.AllPorts()) {
		t.Errorf("GET /api/ports returned %d ports, want %d", len(ports), len(ais.AllPorts()))
	}
}

func TestPortsEndpoint_QuerySearches(t *testing.T) {
	st, _ := newTestApp()
	req := httptest.NewRequest("GET", "/api/ports?q=rotterdam", nil)
	resp, err := New(st, hub.New()).Test(req)
	if err != nil {
		t.Fatal(err)
	}
	var ports []ais.Port
	if err := json.NewDecoder(resp.Body).Decode(&ports); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range ports {
		if p.Locode == "NLRTM" {
			found = true
		}
	}
	if !found {
		t.Errorf("GET /api/ports?q=rotterdam = %v, want it to include NLRTM", ports)
	}
}
