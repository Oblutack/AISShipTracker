package feed

import (
	"testing"

	"github.com/Oblutack/AISShipTracker/internal/ais"
)

// These tests confirm handleMessage correctly extracts fields from a
// payload shaped like the envelope/positionReportEnvelope/
// shipStaticDataEnvelope structs in aisstream.go. They do NOT prove those
// structs match aisstream.io's real wire format - there was no API key
// available to capture a real payload while writing this. Once you have a
// key, log one raw message (see the commented-out line in handleMessage)
// and replace the JSON below with it if the fields don't line up.

func TestHandleMessage_PositionReport(t *testing.T) {
	raw := []byte(`{
		"MessageType": "PositionReport",
		"MetaData": {"MMSI": 257123456, "ShipName": "TEST VESSEL", "latitude": 60.1, "longitude": 5.3, "time_utc": "2026-01-01T00:00:00Z"},
		"Message": {
			"PositionReport": {
				"UserID": 257123456,
				"Cog": 91.2,
				"Sog": 12.5,
				"TrueHeading": 90,
				"Latitude": 60.123,
				"Longitude": 5.456,
				"NavigationalStatus": 0
			}
		}
	}`)

	var got ais.PositionUpdate
	handleMessage(raw, Handler{
		OnPosition: func(u ais.PositionUpdate) { got = u },
	})

	if got.MMSI != 257123456 {
		t.Errorf("MMSI = %d, want 257123456", got.MMSI)
	}
	if got.Latitude != 60.123 || got.Longitude != 5.456 {
		t.Errorf("lat/lon = %v/%v, want 60.123/5.456", got.Latitude, got.Longitude)
	}
	if got.Sog != 12.5 {
		t.Errorf("Sog = %v, want 12.5", got.Sog)
	}
	if got.NavStatus != 0 {
		t.Errorf("NavStatus = %d, want 0", got.NavStatus)
	}
}

// The Dimension/MaximumStaticDraught values here are copied from a real
// message captured off the live aisstream.io feed (a small Norwegian
// vessel, "ABBE") - not hand-invented.
func TestHandleMessage_ShipStaticData(t *testing.T) {
	raw := []byte(`{
		"MessageType": "ShipStaticData",
		"MetaData": {"MMSI": 257123456, "ShipName": "TEST VESSEL", "latitude": 0, "longitude": 0, "time_utc": "2026-01-01T00:00:00Z"},
		"Message": {
			"ShipStaticData": {
				"UserID": 257123456,
				"ImoNumber": 9123456,
				"Name": "TEST VESSEL",
				"CallSign": "LA1234",
				"Type": 70,
				"Destination": "ROTTERDAM",
				"MaximumStaticDraught": 2.5,
				"Dimension": {"A": 11, "B": 8, "C": 4, "D": 3}
			}
		}
	}`)

	var got ais.StaticUpdate
	handleMessage(raw, Handler{
		OnStatic: func(u ais.StaticUpdate) { got = u },
	})

	if got.MMSI != 257123456 {
		t.Errorf("MMSI = %d, want 257123456", got.MMSI)
	}
	if got.Name != "TEST VESSEL" {
		t.Errorf("Name = %q, want %q", got.Name, "TEST VESSEL")
	}
	if got.ShipType != 70 {
		t.Errorf("ShipType = %d, want 70", got.ShipType)
	}
	if got.Destination != "ROTTERDAM" {
		t.Errorf("Destination = %q, want %q", got.Destination, "ROTTERDAM")
	}
	if got.LengthMeters != 19 { // A(11) + B(8)
		t.Errorf("LengthMeters = %v, want 19", got.LengthMeters)
	}
	if got.BeamMeters != 7 { // C(4) + D(3)
		t.Errorf("BeamMeters = %v, want 7", got.BeamMeters)
	}
	if got.DraughtMeters != 2.5 {
		t.Errorf("DraughtMeters = %v, want 2.5", got.DraughtMeters)
	}
}

// This payload shape is copied from a real message captured off the live
// aisstream.io feed (unlike the other tests here, which were hand-built
// from documentation before that was possible) - Class B units genuinely
// omit NavigationalStatus entirely.
func TestHandleMessage_StandardClassBPositionReport(t *testing.T) {
	raw := []byte(`{
		"MessageType": "StandardClassBPositionReport",
		"MetaData": {"MMSI": 227005680, "ShipName": "PILOTINE I", "latitude": 42.69, "longitude": 9.45, "time_utc": "2026-01-01T00:00:00Z"},
		"Message": {
			"StandardClassBPositionReport": {
				"UserID": 227005680,
				"Cog": 63,
				"Sog": 6,
				"TrueHeading": 511,
				"Latitude": 42.697,
				"Longitude": 9.457,
				"PositionAccuracy": true,
				"Valid": true
			}
		}
	}`)

	var got ais.PositionUpdate
	handleMessage(raw, Handler{
		OnPosition: func(u ais.PositionUpdate) { got = u },
	})

	if got.MMSI != 227005680 {
		t.Errorf("MMSI = %d, want 227005680", got.MMSI)
	}
	if got.Sog != 6 {
		t.Errorf("Sog = %v, want 6", got.Sog)
	}
	if got.NavStatus != 15 {
		t.Errorf("NavStatus = %d, want 15 (Not defined - Class B doesn't report this)", got.NavStatus)
	}
}

func TestHandleMessage_UnknownTypeIsIgnored(t *testing.T) {
	raw := []byte(`{"MessageType": "SomethingElse", "MetaData": {}, "Message": {}}`)
	called := false
	handleMessage(raw, Handler{
		OnPosition: func(ais.PositionUpdate) { called = true },
		OnStatic:   func(ais.StaticUpdate) { called = true },
	})
	if called {
		t.Error("handler should not be called for an unrecognized MessageType")
	}
}
