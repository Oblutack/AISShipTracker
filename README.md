# AIS Ship Tracker

Decodes real-time AIS (Automatic Identification System) ship-tracking
messages from a live feed and plots vessels on a map as they move.

No radio hardware involved — it connects to [aisstream.io](https://aisstream.io),
a free aggregator that collects real AIS traffic from a global network of
receivers and streams it over a WebSocket as JSON. This service parses that
stream into a clean internal model, keeps a live in-memory picture of every
vessel currently reporting, and serves it to a browser over its own
WebSocket + a small Leaflet map. Click a vessel to see its real recorded
trail and a best-effort inferred route to its destination.

## What's real, what's inferred

- **Position, speed, course, heading, navigational status** — from AIS
  position reports, broadcast by the vessel itself. Real.
- **Name, callsign, IMO number, ship type, destination** — from AIS
  static/voyage data messages. Real, but only known once that message type
  has been received for a given vessel (it's broadcast far less often than
  position reports).
- **Flag** — derived locally from the vessel's MMSI (the first 3 digits are
  an ITU-assigned country code). Not part of the AIS message itself.
- **Length, beam, draught** — from AIS static/voyage data. Length/beam are
  derived from the four GPS-antenna offsets AIS actually reports (A/B/C/D -
  antenna-to-bow/stern/port/starboard), not sent as a single number.
  Confirmed against live traffic. Absent (omitted from JSON) for smaller
  craft that don't report it, which is common - not every vessel does.
- **Ports** — `GET /api/ports` serves the same real UN/LOCODE dataset the
  destination matcher uses (~11,800 ports), independent of anything
  currently being tracked. `?q=` does a substring search on name/LOCODE.
- **Position trail** (`GET /api/vessels/:mmsi/track`) — real: the vessel's
  actual reported positions over time, throttled to one point per ~30s and
  capped at ~120 points per vessel.
- **Route to destination** (`GET /api/vessels/:mmsi/route`) — **inferred,
  not real**. AIS has no voyage-plan concept at all - there is no "route" in
  the protocol. This best-effort matches the vessel's free-text Destination
  field against the official UN/LOCODE port dataset and, if matched, draws
  a straight great-circle line to it - not ocean-routed, doesn't avoid
  land, just a geometric estimate. Labeled as such everywhere it appears
  (API response, map popup). See `internal/ais/ports.go` for the matching
  logic and its two known, documented gaps (some real ports have no
  coordinates in the source data at all; some are recorded under a native-
  language name that won't match an English destination string). Measured
  match rate against real captured traffic: ~38%.
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
   `http://localhost:8090/api/vessels` for a JSON snapshot. Click any
   vessel to draw its trail (solid blue) and inferred route (dashed amber,
   if its destination matched a known port).

## Architecture

```
internal/
├── ais/       # domain model (Vessel) + lookup tables (MID→flag, AIS
│   └── data/  # ship-type codes, nav-status codes) + UN/LOCODE-backed
│              # destination matching (ports.go) + great-circle math
├── feed/      # connects to aisstream.io, decodes its JSON messages into
│              # ais.PositionUpdate / ais.StaticUpdate, auto-reconnects
├── store/     # thread-safe in-memory cache: latest state + position
│              # history per MMSI
├── hub/       # WebSocket fan-out to connected browser clients
└── httpapi/   # Fiber server: /health, /api/vessels, /api/vessels/:mmsi/
               # {track,route}, /api/ports, /ws, static map page
web/
└── index.html # single-file Leaflet map, no build step
```

Storage is in-memory only for now — a live tracker mostly cares about
"where is this ship right now," so there's no database to stand up before
you see a real ship move. `store.Store`'s method set is small and
self-contained enough that adding Postgres/PostGIS-backed persistence later
(for durability across restarts, or trails longer than the current ~1hr
cap) means writing a second implementation, not touching any caller.

## The aisstream.io message schema — verified against real traffic

`internal/feed/aisstream.go` parses aisstream.io's JSON messages. The
`PositionReport`, `ShipStaticData`, `StandardClassBPositionReport`, and
`StaticDataReport` struct definitions have all been confirmed
field-for-field against real live traffic (thousands of messages, zero
parse errors). If you extend this and vessels aren't parsing as expected,
uncomment the raw-payload log line in `handleMessage` and compare against
the struct tags - a fix stays isolated to that one file, nothing
downstream depends on this wire format.

`StaticDataReport` (Class B's split Part A/Part B static data, ~8% of
real live traffic) arrives as two separate messages per MMSI - a Part A
carrying only the vessel's name, a Part B carrying type/callsign/
dimensions, never both at once. Rather than buffer and join the two
halves in the feed layer, each part is emitted as its own partial
`ais.StaticUpdate` (only the fields that part actually carries are set)
and left to `store.ApplyStatic`'s existing per-field merge to assemble
over time - the same mechanism that already lets position and static
updates arrive independently without clobbering each other. This needed
one real fix in `store.go`: `ShipType` and `Destination` used to be
written unconditionally on every static update, which was harmless when
every update came from one `ShipStaticData` message carrying every field
at once, but would have let a lone Part A silently zero out a vessel's
already-known type. Both fields now follow the same "zero/empty means
unknown, don't overwrite" rule the dimension fields already used.

## Companion project

This is a standalone service — separate purpose, separate tech surface,
separate repo from [NeptuneShipments](https://github.com/Oblutack/NeptuneShipments)
(a simulated fleet-management ERP). NeptuneShipments' map has an optional
"AIS Live" layer that connects directly to this service's `/ws` feed and
renders real ships alongside its simulated fleet - no shared database, no
shared git history, just a WebSocket connection. This service has to be
running for that layer to show anything.
