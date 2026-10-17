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

// These two payloads were captured directly off the live aisstream.io
// feed (not hand-built from the generated model docs) - a real Part A
// for MMSI 368122880 ("QUEEN CITY") and a real Part B for MMSI
// 244039283. Confirms the Valid flags, not PartNumber, are what actually
// says which half of the struct has real data - both ReportA and ReportB
// are always present in the JSON, with the inactive half zeroed out.
func TestHandleMessage_StaticDataReport_PartA(t *testing.T) {
	raw := []byte(`{
		"MessageType": "StaticDataReport",
		"MetaData": {"MMSI": 368122880, "ShipName": "QUEEN CITY", "latitude": 48.75618, "longitude": -122.49938, "time_utc": "2026-10-17T16:08:49Z"},
		"Message": {
			"StaticDataReport": {
				"MessageID": 24,
				"PartNumber": false,
				"UserID": 368122880,
				"Valid": true,
				"ReportA": {"Valid": true, "Name": "QUEEN CITY"},
				"ReportB": {"Valid": false, "ShipType": 0, "CallSign": "", "Dimension": {"A": 0, "B": 0, "C": 0, "D": 0}}
			}
		}
	}`)

	var got ais.StaticUpdate
	handleMessage(raw, Handler{
		OnStatic: func(u ais.StaticUpdate) { got = u },
	})

	if got.MMSI != 368122880 {
		t.Errorf("MMSI = %d, want 368122880", got.MMSI)
	}
	if got.Name != "QUEEN CITY" {
		t.Errorf("Name = %q, want %q", got.Name, "QUEEN CITY")
	}
	if got.CallSign != "" || got.ShipType != 0 || got.LengthMeters != 0 || got.BeamMeters != 0 {
		t.Errorf("Part A update should leave ReportB-only fields unset, got %+v", got)
	}
}

func TestHandleMessage_StaticDataReport_PartB(t *testing.T) {
	raw := []byte(`{
		"MessageType": "StaticDataReport",
		"MetaData": {"MMSI": 244039283, "ShipName": "", "latitude": 52.32949, "longitude": 5.54796, "time_utc": "2026-10-17T16:08:49Z"},
		"Message": {
			"StaticDataReport": {
				"MessageID": 24,
				"PartNumber": true,
				"UserID": 244039283,
				"Valid": true,
				"ReportA": {"Valid": false, "Name": ""},
				"ReportB": {"Valid": true, "ShipType": 36, "CallSign": "PH8769", "Dimension": {"A": 9, "B": 3, "C": 2, "D": 2}}
			}
		}
	}`)

	var got ais.StaticUpdate
	handleMessage(raw, Handler{
		OnStatic: func(u ais.StaticUpdate) { got = u },
	})

	if got.MMSI != 244039283 {
		t.Errorf("MMSI = %d, want 244039283", got.MMSI)
	}
	if got.Name != "" {
		t.Errorf("Part B update should leave Name unset, got %q", got.Name)
	}
	if got.CallSign != "PH8769" {
		t.Errorf("CallSign = %q, want %q", got.CallSign, "PH8769")
	}
	if got.ShipType != 36 {
		t.Errorf("ShipType = %d, want 36", got.ShipType)
	}
	if got.LengthMeters != 12 { // A(9) + B(3)
		t.Errorf("LengthMeters = %v, want 12", got.LengthMeters)
	}
	if got.BeamMeters != 4 { // C(2) + D(2)
		t.Errorf("BeamMeters = %v, want 4", got.BeamMeters)
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
