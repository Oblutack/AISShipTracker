// Package feed connects to aisstream.io's live WebSocket feed and turns its
// JSON messages into ais.PositionUpdate / ais.StaticUpdate values.
//
// IMPORTANT - read before debugging a "nothing is happening" symptom:
// the envelope/positionReportEnvelope/shipStaticDataEnvelope structs below
// reflect aisstream.io's documented JSON schema as best understood at the
// time this was written, and have NOT been verified against a live
// connection (no API key was available while scaffolding this). If you
// connect with a real key and updates aren't arriving, the first thing to
// check is what the raw payload actually looks like - uncomment the
// log.Printf("feed: raw: %s", raw) line in handleMessage, compare it
// against the struct tags here, and fix the tags. Everything downstream
// (store, hub, API, map) does not care about this format at all, so a fix
// here is a one-file change.
package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Oblutack/AISShipTracker/internal/ais"
	"github.com/gorilla/websocket"
)

const streamURL = "wss://stream.aisstream.io/v0/stream"

// Config controls what the feed subscribes to.
type Config struct {
	APIKey string
	// BoundingBoxes restricts the subscription to one or more
	// [[minLat,minLon],[maxLat,maxLon]] boxes. aisstream.io requires at
	// least one; use [][2][2]float64{{{-90, -180}, {90, 180}}} for the
	// whole world.
	BoundingBoxes [][2][2]float64
	// FilterMessageTypes optionally restricts which AIS message types are
	// streamed, e.g. []string{"PositionReport", "ShipStaticData"}. Nil/empty
	// means "all types" (a lot more volume - static data is a tiny fraction
	// of traffic compared to position reports).
	FilterMessageTypes []string
}

// Handler receives decoded updates as they arrive. Either field may be nil
// if the caller doesn't care about that message kind.
type Handler struct {
	OnPosition func(ais.PositionUpdate)
	OnStatic   func(ais.StaticUpdate)
}

// Run connects to aisstream.io and dispatches decoded messages to h until
// ctx is canceled. It reconnects automatically with exponential backoff on
// any read/dial error - long-lived streaming connections drop routinely,
// that's expected, not a fatal condition.
func Run(ctx context.Context, cfg Config, h Handler) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := connectOnce(ctx, cfg, h)
		if err != nil && ctx.Err() == nil {
			log.Printf("feed: connection error: %v (retrying in %s)", err, backoff)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func connectOnce(ctx context.Context, cfg Config, h Handler) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, streamURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	sub := subscriptionRequest{
		APIKey:             cfg.APIKey,
		BoundingBoxes:      cfg.BoundingBoxes,
		FilterMessageTypes: cfg.FilterMessageTypes,
	}
	if err := conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	log.Println("feed: subscribed to aisstream.io")

	// Reset backoff behavior on a successful connection by returning nil
	// only via ctx cancellation; any read error below still triggers a
	// backoff retry from Run's caller.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		handleMessage(data, h)
	}
}

type subscriptionRequest struct {
	APIKey             string          `json:"APIKey"`
	BoundingBoxes      [][2][2]float64 `json:"BoundingBoxes"`
	FilterMessageTypes []string        `json:"FilterMessageTypes,omitempty"`
}

// envelope is the outer shape of every message aisstream.io sends.
type envelope struct {
	MessageType string          `json:"MessageType"`
	MetaData    metaData        `json:"MetaData"`
	Message     json.RawMessage `json:"Message"`
}

type metaData struct {
	MMSI      int     `json:"MMSI"`
	ShipName  string  `json:"ShipName"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	TimeUTC   string  `json:"time_utc"`
}

type positionReportEnvelope struct {
	PositionReport struct {
		UserID             int     `json:"UserID"`
		Cog                float64 `json:"Cog"`
		Sog                float64 `json:"Sog"`
		TrueHeading        int     `json:"TrueHeading"`
		Latitude           float64 `json:"Latitude"`
		Longitude          float64 `json:"Longitude"`
		NavigationalStatus int     `json:"NavigationalStatus"`
	} `json:"PositionReport"`
}

type shipStaticDataEnvelope struct {
	ShipStaticData struct {
		UserID               int     `json:"UserID"`
		ImoNumber            int     `json:"ImoNumber"`
		Name                 string  `json:"Name"`
		CallSign             string  `json:"CallSign"`
		Type                 int     `json:"Type"`
		Destination          string  `json:"Destination"`
		MaximumStaticDraught float64 `json:"MaximumStaticDraught"`
		// Dimension gives the vessel's size as four offsets from its GPS
		// antenna (ITU-R M.1371): A = antenna-to-bow, B = antenna-to-stern,
		// C = antenna-to-port-side, D = antenna-to-starboard-side. So
		// length overall = A+B, beam = C+D. Confirmed against live
		// traffic (e.g. a real vessel with A:11 B:8 C:4 D:3 -> 19m x 7m,
		// a plausible small craft).
		Dimension struct {
			A int `json:"A"`
			B int `json:"B"`
			C int `json:"C"`
			D int `json:"D"`
		} `json:"Dimension"`
	} `json:"ShipStaticData"`
}

// standardClassBPositionReportEnvelope covers Class B transceivers - the
// smaller units used by pleasure craft, fishing boats, tenders, etc.
// Confirmed against live traffic to carry no NavigationalStatus field at
// all (Class B units don't report it); PositionUpdate.NavStatus is set to
// 15 ("Not defined") for these rather than defaulting to 0 ("Under way
// using engine"), which would be a false claim.
type standardClassBPositionReportEnvelope struct {
	StandardClassBPositionReport struct {
		UserID      int     `json:"UserID"`
		Cog         float64 `json:"Cog"`
		Sog         float64 `json:"Sog"`
		TrueHeading int     `json:"TrueHeading"`
		Latitude    float64 `json:"Latitude"`
		Longitude   float64 `json:"Longitude"`
	} `json:"StandardClassBPositionReport"`
}

// staticDataReportEnvelope covers AIS message type 24 (Class B static
// data), which - unlike ShipStaticData's single message carrying every
// field - arrives as two separate halves per MMSI: Part A (ReportA, name
// only) and Part B (ReportB, type/callsign/dimensions only), never both
// at once. Confirmed against live traffic: the inactive half is present
// in the JSON but zero-valued with Valid:false, so Valid (not PartNumber)
// is what actually tells you whether a given half's fields mean anything.
// AIS type 24 carries no Destination field at all - that only comes from
// ShipStaticData (type 5).
type staticDataReportEnvelope struct {
	StaticDataReport struct {
		UserID  int `json:"UserID"`
		ReportA struct {
			Valid bool   `json:"Valid"`
			Name  string `json:"Name"`
		} `json:"ReportA"`
		ReportB struct {
			Valid     bool   `json:"Valid"`
			ShipType  int    `json:"ShipType"`
			CallSign  string `json:"CallSign"`
			Dimension struct {
				A int `json:"A"`
				B int `json:"B"`
				C int `json:"C"`
				D int `json:"D"`
			} `json:"Dimension"`
		} `json:"ReportB"`
	} `json:"StaticDataReport"`
}

func handleMessage(raw []byte, h Handler) {
	// Uncomment while debugging a schema mismatch:
	// log.Printf("feed: raw: %s", raw)

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		log.Printf("feed: malformed envelope: %v", err)
		return
	}

	now := time.Now()

	switch env.MessageType {
	case "PositionReport":
		var pr positionReportEnvelope
		if err := json.Unmarshal(env.Message, &pr); err != nil {
			log.Printf("feed: malformed PositionReport: %v", err)
			return
		}
		mmsi := pr.PositionReport.UserID
		if mmsi == 0 {
			mmsi = env.MetaData.MMSI
		}
		if h.OnPosition != nil {
			h.OnPosition(ais.PositionUpdate{
				MMSI:       mmsi,
				Latitude:   pr.PositionReport.Latitude,
				Longitude:  pr.PositionReport.Longitude,
				Cog:        pr.PositionReport.Cog,
				Sog:        pr.PositionReport.Sog,
				Heading:    pr.PositionReport.TrueHeading,
				NavStatus:  pr.PositionReport.NavigationalStatus,
				ReceivedAt: now,
			})
		}

	case "ShipStaticData":
		var sd shipStaticDataEnvelope
		if err := json.Unmarshal(env.Message, &sd); err != nil {
			log.Printf("feed: malformed ShipStaticData: %v", err)
			return
		}
		mmsi := sd.ShipStaticData.UserID
		if mmsi == 0 {
			mmsi = env.MetaData.MMSI
		}
		if h.OnStatic != nil {
			dim := sd.ShipStaticData.Dimension
			h.OnStatic(ais.StaticUpdate{
				MMSI:          mmsi,
				IMO:           sd.ShipStaticData.ImoNumber,
				Name:          sd.ShipStaticData.Name,
				CallSign:      sd.ShipStaticData.CallSign,
				ShipType:      sd.ShipStaticData.Type,
				Destination:   sd.ShipStaticData.Destination,
				LengthMeters:  float64(dim.A + dim.B),
				BeamMeters:    float64(dim.C + dim.D),
				DraughtMeters: sd.ShipStaticData.MaximumStaticDraught,
				ReceivedAt:    now,
			})
		}

	case "StandardClassBPositionReport":
		var pr standardClassBPositionReportEnvelope
		if err := json.Unmarshal(env.Message, &pr); err != nil {
			log.Printf("feed: malformed StandardClassBPositionReport: %v", err)
			return
		}
		mmsi := pr.StandardClassBPositionReport.UserID
		if mmsi == 0 {
			mmsi = env.MetaData.MMSI
		}
		if h.OnPosition != nil {
			h.OnPosition(ais.PositionUpdate{
				MMSI:       mmsi,
				Latitude:   pr.StandardClassBPositionReport.Latitude,
				Longitude:  pr.StandardClassBPositionReport.Longitude,
				Cog:        pr.StandardClassBPositionReport.Cog,
				Sog:        pr.StandardClassBPositionReport.Sog,
				Heading:    pr.StandardClassBPositionReport.TrueHeading,
				NavStatus:  15, // "Not defined" - Class B doesn't report this
				ReceivedAt: now,
			})
		}

	case "StaticDataReport":
		var sdr staticDataReportEnvelope
		if err := json.Unmarshal(env.Message, &sdr); err != nil {
			log.Printf("feed: malformed StaticDataReport: %v", err)
			return
		}
		rep := sdr.StaticDataReport
		mmsi := rep.UserID
		if mmsi == 0 {
			mmsi = env.MetaData.MMSI
		}
		if h.OnStatic != nil {
			// Only one of ReportA/ReportB is actually populated per
			// message (see the envelope's doc comment) - leave the
			// other half's fields at their zero value. ApplyStatic
			// treats zero/empty as "unknown, don't overwrite" for
			// every one of these fields, so this partial update merges
			// correctly with whatever's already known for this vessel
			// instead of clobbering it.
			u := ais.StaticUpdate{MMSI: mmsi, ReceivedAt: now}
			if rep.ReportA.Valid {
				u.Name = rep.ReportA.Name
			}
			if rep.ReportB.Valid {
				u.CallSign = rep.ReportB.CallSign
				u.ShipType = rep.ReportB.ShipType
				u.LengthMeters = float64(rep.ReportB.Dimension.A + rep.ReportB.Dimension.B)
				u.BeamMeters = float64(rep.ReportB.Dimension.C + rep.ReportB.Dimension.D)
			}
			h.OnStatic(u)
		}

	default:
		// Other message types (e.g. AidsToNavigationReport) aren't
		// handled yet - safe to ignore for now.
	}
}
