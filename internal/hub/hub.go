// Package hub broadcasts vessel updates to every connected browser client
// over WebSocket. One Hub per server process; clients register/unregister
// themselves and the Run loop fans out every broadcast message to all of
// them without blocking on a slow client.
package hub

import "log"

type Client struct {
	Send chan []byte
}

func NewClient() *Client {
	return &Client{Send: make(chan []byte, 32)}
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func New() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }
func (h *Hub) Broadcast(msg []byte) { h.broadcast <- msg }

// Run drives the hub's event loop. Call it once, in its own goroutine,
// before any client registers.
func (h *Hub) Run() {
	log.Println("hub: running")
	for {
		select {
		case c := <-h.register:
			h.clients[c] = true
			log.Printf("hub: client connected (total: %d)", len(h.clients))

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.Send)
			}
			log.Printf("hub: client disconnected (total: %d)", len(h.clients))

		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.Send <- msg:
				default:
					// Client's buffer is full - drop it rather than block
					// the whole hub on one slow reader.
					delete(h.clients, c)
					close(c.Send)
				}
			}
		}
	}
}
