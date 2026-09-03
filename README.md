# AIS Ship Tracker

Decodes real-time AIS (Automatic Identification System) ship-tracking
messages from a live feed and plots vessels on a map as they move.

No radio hardware involved — it connects to [aisstream.io](https://aisstream.io),
a free aggregator that collects real AIS traffic from a global network of
receivers and streams it over a WebSocket as JSON. This service parses that
stream into a clean internal model, keeps a live in-memory picture of every
vessel currently reporting, and serves it to a browser over its own
WebSocket + a small Leaflet map.

## What's real, what's inferred

- **Position, speed, course, heading, navigational status** — from AIS
  position reports, broadcast by the vessel itself. Real.
- **Name, callsign, IMO number, ship type, destination** — from AIS
  static/voyage data messages. Real, but only known once that message type
  has been received for a given vessel (it's broadcast far less often than
  position reports).
- **Flag** — derived locally from the vessel's MMSI (the first 3 digits are
  an ITU-assigned country code). Not part of the AIS message itself.
- **Cargo contents** — not available from any public source, real or
  otherwise. AIS only reports a coarse ship-*type* category (e.g. "Tanker"),
  never what's actually aboard. See `ais.InferredCargoLabel` if you want a
  clearly-labeled guess based on that category — it is explicitly not real
  data.

## Setup

1. Get a free API key at aisstream.io (instant, no approval wait).
2. `cp .env.example .env` and fill in `AISSTREAM_API_KEY`. Optionally set a
   bounding box (`BBOX_MIN_LAT` etc.) to limit volume to one region —
   without one, you subscribe to the entire world's AIS traffic, which is
   a lot of messages.
3. `go run cmd/server/main.go`
4. Open `http://localhost:8090/` for the live map, or hit
   `http://localhost:8090/api/vessels` for a JSON snapshot.

## Architecture

```
internal/
├── ais/       # domain model (Vessel) + static lookup tables (MID→flag,
│              # ship-type codes, nav-status codes) - no external deps
├── feed/      # connects to aisstream.io, decodes its JSON messages into
│              # ais.PositionUpdate / ais.StaticUpdate, auto-reconnects
├── store/     # thread-safe in-memory cache: latest known state per MMSI
├── hub/       # WebSocket fan-out to connected browser clients
└── httpapi/   # Fiber server: /health, /api/vessels, /ws, static map page
web/
└── index.html # single-file Leaflet map, no build step
```

Storage is in-memory only for now — a live tracker mostly cares about
"where is this ship right now," so there's no database to stand up before
you see a real ship move. `store.Store`'s method set is small and
self-contained enough that adding Postgres/PostGIS-backed persistence later
(for historical track playback, durability across restarts) means writing a
second implementation, not touching any caller.

## A known unknown: the aisstream.io message schema

`internal/feed/aisstream.go` parses aisstream.io's JSON messages using
struct definitions based on their documented schema. That schema has **not
been verified against live traffic** — this was scaffolded without an API
key. If you run this with a real key and vessels aren't showing up, that
file is the first place to look: uncomment the raw-payload log line in
`handleMessage`, compare what actually arrives against the struct tags, and
adjust. Nothing downstream (the store, hub, API, map) depends on this
format at all, so a fix is isolated to that one file. `go test ./...`
covers the parsing logic itself (i.e. "does this code correctly extract
fields from JSON shaped like X"), just not whether X is what the API
actually sends.

## Companion project

This is a standalone service — separate purpose, separate tech surface,
separate repo from [NeptuneShipments](https://github.com/Oblutack/NeptuneShipments)
(a simulated fleet-management ERP). The two can optionally be wired
together later: this service's `/ws` feed could be consumed by
NeptuneShipments' map as a "live traffic" overlay alongside its simulated
fleet, without sharing a database or git history.
