// Package ais holds the decoded, application-level view of a vessel - the
// shape everything else (feed parsers, the store, the API) is built around.
// It is deliberately independent of any single feed provider's wire format.
package ais

import "time"

// Vessel is the latest known state of one ship, keyed by MMSI.
// Fields are populated incrementally: a PositionReport fills the movement
// fields, a ShipStaticData message fills the identity fields. A vessel may
// have position data long before (or without) static data ever arriving.
type Vessel struct {
	MMSI      int    `json:"mmsi"`
	IMO       int    `json:"imo,omitempty"`
	Name      string `json:"name,omitempty"`
	CallSign  string `json:"call_sign,omitempty"`
	ShipType  int    `json:"ship_type,omitempty"`
	TypeLabel string `json:"type_label,omitempty"`
	Flag      string `json:"flag,omitempty"`

	Destination string `json:"destination,omitempty"`

	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// Cog: course over ground, degrees. Sog: speed over ground, knots.
	Cog             float64 `json:"cog"`
	Sog             float64 `json:"sog"`
	Heading         int     `json:"heading"`
	NavStatus       int     `json:"nav_status"`
	NavStatusLabel  string  `json:"nav_status_label,omitempty"`

	HasPosition bool      `json:"has_position"`
	HasStatic   bool      `json:"has_static"`
	LastUpdate  time.Time `json:"last_update"`
}

// PositionUpdate carries only the fields a position report can populate.
// Kept separate from Vessel so the store can merge partial updates without
// clobbering identity fields it doesn't know about yet.
type PositionUpdate struct {
	MMSI       int
	Latitude   float64
	Longitude  float64
	Cog        float64
	Sog        float64
	Heading    int
	NavStatus  int
	ReceivedAt time.Time
}

// StaticUpdate carries only the fields a static/voyage data report can populate.
type StaticUpdate struct {
	MMSI        int
	IMO         int
	Name        string
	CallSign    string
	ShipType    int
	Destination string
	ReceivedAt  time.Time
}
