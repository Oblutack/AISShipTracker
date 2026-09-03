// Package httpapi exposes the store over HTTP: a REST snapshot endpoint,
// a WebSocket for live updates, and the static map page.
package httpapi

import (
	"encoding/json"
	"strconv"

	"github.com/Oblutack/AISShipTracker/internal/ais"
	"github.com/Oblutack/AISShipTracker/internal/hub"
	"github.com/Oblutack/AISShipTracker/internal/store"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
)

// New builds the Fiber app. h must already be running (h.Run() in its own
// goroutine) before any request arrives.
func New(st *store.Store, h *hub.Hub) *fiber.App {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET",
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Snapshot of everything currently known - useful for a fresh page
	// load, or anything that just wants a one-shot list without opening
	// a WebSocket.
	app.Get("/api/vessels", func(c *fiber.Ctx) error {
		return c.JSON(st.List())
	})

	// Real position history for one vessel - the actual reported track,
	// not a prediction. Fetched on-demand (not pushed over /ws) since
	// broadcasting every vessel's full history on every tick would be
	// enormous; the frontend fetches this only for a vessel the user
	// has selected.
	app.Get("/api/vessels/:mmsi/track", func(c *fiber.Ctx) error {
		mmsi, err := strconv.Atoi(c.Params("mmsi"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mmsi must be a number"})
		}
		return c.JSON(st.Track(mmsi))
	})

	// Best-effort INFERRED route from a vessel's current position to a
	// destination guessed from its self-reported (free-text, unvalidated)
	// AIS Destination field. This is not real data - see
	// ais.InferredRouteNotice, also included in the response so any
	// consumer has the disclaimer without needing to know to ask for it.
	app.Get("/api/vessels/:mmsi/route", func(c *fiber.Ctx) error {
		mmsi, err := strconv.Atoi(c.Params("mmsi"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mmsi must be a number"})
		}
		v, ok := st.Get(mmsi)
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "unknown vessel"})
		}
		if !v.HasPosition {
			// A vessel can exist in the store from a static/voyage data
			// message alone, before any position report ever arrives -
			// Latitude/Longitude are the zero value (0,0 - "null island")
			// in that case, not a real position. Using it as a route
			// origin would silently draw a nonsense line from the Gulf
			// of Guinea.
			return c.JSON(fiber.Map{"matched": false, "reason": "vessel has no reported position yet"})
		}
		if v.Destination == "" {
			return c.JSON(fiber.Map{"matched": false, "reason": "vessel has not reported a destination"})
		}
		port, ok := ais.MatchDestination(v.Destination)
		if !ok {
			return c.JSON(fiber.Map{
				"matched":          false,
				"reason":           "reported destination text did not match any known port",
				"destination_text": v.Destination,
			})
		}
		path := ais.GreatCirclePath(v.Latitude, v.Longitude, port.Lat, port.Lon, 60)
		return c.JSON(fiber.Map{
			"matched":          true,
			"notice":           ais.InferredRouteNotice,
			"destination_text": v.Destination,
			"destination_port": port,
			"path":             path,
		})
	})

	// Real global port data - the same UN/LOCODE dataset the destination
	// matcher uses (internal/ais/ports.go), not tied to anything currently
	// being tracked. Free, no external API key, ~11,800 real ports. With
	// no query it returns the full list (~700KB; static per process
	// lifetime, hence the cache header); with ?q= it does a substring
	// search over name/LOCODE, capped at 50 results.
	app.Get("/api/ports", func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "public, max-age=86400")
		if q := c.Query("q"); q != "" {
			return c.JSON(ais.SearchPorts(q))
		}
		return c.JSON(ais.AllPorts())
	})

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		client := hub.NewClient()
		h.Register(client)
		defer h.Unregister(client)

		// Send the current snapshot immediately so a new client doesn't
		// have to wait for the next live update to see anything.
		if snapshot, err := json.Marshal(fiber.Map{
			"type": "SNAPSHOT",
			"data": st.List(),
		}); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, snapshot)
		}

		// Writer goroutine: hub -> socket.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for msg := range client.Send {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			}
		}()

		// Reader: we don't expect messages from the client, but reading
		// is what detects the connection closing so we can clean up.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
		<-done
	}))

	app.Static("/", "./web")

	return app
}
