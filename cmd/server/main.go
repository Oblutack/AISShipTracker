package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Oblutack/AISShipTracker/internal/ais"
	"github.com/Oblutack/AISShipTracker/internal/feed"
	"github.com/Oblutack/AISShipTracker/internal/hub"
	"github.com/Oblutack/AISShipTracker/internal/httpapi"
	"github.com/Oblutack/AISShipTracker/internal/store"
	"github.com/joho/godotenv"
)

// wsMessage is the envelope every message pushed to /ws clients uses.
type wsMessage struct {
	Type string      `json:"type"`
	Data ais.Vessel  `json:"data"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, relying on real environment variables")
	}

	apiKey := os.Getenv("AISSTREAM_API_KEY")
	if apiKey == "" {
		log.Fatal("AISSTREAM_API_KEY is not set - get a free key at aisstream.io and put it in .env")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	bbox := parseBoundingBox()

	st := store.New()
	h := hub.New()
	go h.Run()

	// Periodically drop vessels we haven't heard from in a while, so the
	// store (and every snapshot/broadcast) doesn't grow forever.
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			removed := st.Prune(30 * time.Minute)
			if removed > 0 {
				log.Printf("store: pruned %d stale vessel(s)", removed)
			}
		}
	}()

	// Feed: decode messages, merge into the store, broadcast the updated
	// vessel to every connected browser.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		cfg := feed.Config{
			APIKey:        apiKey,
			BoundingBoxes: bbox,
			// Leave FilterMessageTypes empty for now to see everything
			// while verifying the schema against real traffic; narrow it
			// to []string{"PositionReport", "ShipStaticData"} once
			// confirmed working, to cut down on volume.
		}

		handler := feed.Handler{
			OnPosition: func(u ais.PositionUpdate) {
				v := st.ApplyPosition(u)
				broadcast(h, "POSITION_UPDATE", v)
			},
			OnStatic: func(u ais.StaticUpdate) {
				v := st.ApplyStatic(u)
				broadcast(h, "STATIC_UPDATE", v)
			},
		}

		if err := feed.Run(ctx, cfg, handler); err != nil && ctx.Err() == nil {
			log.Printf("feed: stopped: %v", err)
		}
	}()

	app := httpapi.New(st, h)

	go func() {
		log.Printf("AIS Ship Tracker listening on :%s (map at http://localhost:%s/)", port, port)
		if err := app.Listen(":" + port); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on Ctrl+C.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down...")
	cancel()
	_ = app.Shutdown()
}

func broadcast(h *hub.Hub, msgType string, v ais.Vessel) {
	data, err := json.Marshal(wsMessage{Type: msgType, Data: v})
	if err != nil {
		log.Printf("broadcast: marshal error: %v", err)
		return
	}
	h.Broadcast(data)
}

// parseBoundingBox reads BBOX_MIN_LAT/BBOX_MIN_LON/BBOX_MAX_LAT/BBOX_MAX_LON
// from the environment and builds a single-box subscription area. Defaults
// to the whole world if any value is missing or invalid.
func parseBoundingBox() [][2][2]float64 {
	whole := [][2][2]float64{{{-90, -180}, {90, 180}}}

	minLat, err1 := strconv.ParseFloat(os.Getenv("BBOX_MIN_LAT"), 64)
	minLon, err2 := strconv.ParseFloat(os.Getenv("BBOX_MIN_LON"), 64)
	maxLat, err3 := strconv.ParseFloat(os.Getenv("BBOX_MAX_LAT"), 64)
	maxLon, err4 := strconv.ParseFloat(os.Getenv("BBOX_MAX_LON"), 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		log.Println("bounding box env vars not set/invalid - subscribing to the whole world (higher volume)")
		return whole
	}

	return [][2][2]float64{{{minLat, minLon}, {maxLat, maxLon}}}
}
