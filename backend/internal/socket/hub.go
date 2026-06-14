package socket

import "sync"

// InputEvent carries a player's input to the room goroutine via a channel.
type InputEvent struct {
	TankID string
	Input  PlayerInput
}

// Hub holds the communication channels into the room goroutine.
// It does NOT touch game state directly — that belongs exclusively to the room goroutine.
type Hub struct {
	Register   chan *Client
	Unregister chan *Client
	InputCh    chan InputEvent

	// namesMu guards names, which is read from HTTP handler goroutines (IsNameTaken).
	namesMu sync.RWMutex
	names   map[string]bool
}

func NewHub() *Hub {
	return &Hub{
		Register:   make(chan *Client, 8),
		Unregister: make(chan *Client, 8),
		InputCh:    make(chan InputEvent, 256),
		names:      make(map[string]bool),
	}
}

func (h *Hub) IsNameTaken(name string) bool {
	h.namesMu.RLock()
	defer h.namesMu.RUnlock()
	return h.names[name]
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
