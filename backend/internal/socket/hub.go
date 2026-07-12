package socket

import (
	"sync"
	"zhuch/pkg/engine"
)

// InputEvent carries a player's input to the room goroutine via a channel.
type InputEvent struct {
	TankID string
	Input  PlayerInput
}

// Hub holds the communication channels into the room goroutine.
// It does NOT touch game state directly — that belongs exclusively to the room goroutine.
type Hub struct {
	Register     chan *Client
	Unregister   chan *Client
	InputCh      chan InputEvent
	GraceExpired chan string // player name whose grace period expired

	// namesMu guards names, which is read from HTTP handler goroutines (IsNameTaken).
	namesMu sync.RWMutex
	names   map[string]bool
}

func NewHub() *Hub {
	return &Hub{
		Register:     make(chan *Client, 8),
		Unregister:   make(chan *Client, 8),
		InputCh:      make(chan InputEvent, 256),
		GraceExpired: make(chan string, 32),
		names:        make(map[string]bool),
	}
}

func (h *Hub) IsNameTaken(name string) bool {
	h.namesMu.RLock()
	defer h.namesMu.RUnlock()
	return h.names[name]
}

// PlayerCount returns the number of actively connected players (names claimed).
// Safe to call from HTTP handler goroutines.
func (h *Hub) PlayerCount() int {
	h.namesMu.RLock()
	defer h.namesMu.RUnlock()
	return len(h.names)
}

func (h *Hub) claimName(name string) {
	h.namesMu.Lock()
	h.names[name] = true
	h.namesMu.Unlock()
}

func (h *Hub) releaseName(name string) {
	h.namesMu.Lock()
	delete(h.names, name)
	h.namesMu.Unlock()
}

// obstaclePayload flattens Arena.Obstacles to the wire shape for the init
// message: [{"x":..,"y":..,"radius":..}]. Only circle obstacles exist today.
func obstaclePayload(obstacles []engine.GeomObject) []map[string]float64 {
	out := make([]map[string]float64, 0, len(obstacles))
	for _, obs := range obstacles {
		if c, ok := obs.(*engine.Circle); ok {
			out = append(out, map[string]float64{"x": c.Center.X, "y": c.Center.Y, "radius": c.Radius})
		}
	}
	return out
}
