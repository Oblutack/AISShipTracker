// Package httpapi exposes the store over HTTP: a REST snapshot endpoint,
// a WebSocket for live updates, and the static map page.
package httpapi

import (
	"encoding/json"

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
