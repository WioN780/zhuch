package socket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"zhuch/pkg/engine"
)

// Hub connects clients to a game
type Hub struct {
	Game    *engine.Game
	Clients map[*Client]bool // active connections

	// Channels for thread-safe client connections
	Register   chan *Client
	Unregister chan *Client

	mu sync.Mutex
}

func NewHub(g *engine.Game) *Hub {
	return &Hub{
		Game:       g,
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Run starts the hub loop
func (h *Hub) Run() {
	slog.Info("socket hub starting")
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			// Create a tank in the engine!
			tank := engine.NewTank(engine.Vector2{X: 100, Y: 100}, &h.Game.Config)
			client.TankID = tank.GetID()
			h.Game.Entities = append(h.Game.Entities, tank)
			h.mu.Unlock()

			// Send initialization message to the client
			initMsg, _ := json.Marshal(map[string]string{
				"type":    "init",
				"tank_id": client.TankID,
			})
			client.Send <- initMsg

			slog.Info("client registered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(h.Clients))

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)

				// Remove tank from engine
				newEntities := h.Game.Entities[:0]
				for _, e := range h.Game.Entities {
					if e.GetID() != client.TankID {
						newEntities = append(newEntities, e)
					}
				}
				h.Game.Entities = newEntities
				slog.Info("client unregistered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(h.Clients))
			}
			h.mu.Unlock()
		}
	}
}

// This prepares and sends filtered snapshots to each client
func (h *Hub) BroadcastGameState() {
	h.mu.Lock()
	defer h.mu.Unlock()

	tankMap := make(map[string]*engine.Tank)
	for _, e := range h.Game.Entities {
		if t, ok := e.(*engine.Tank); ok {
			tankMap[t.GetID()] = t
		}
	}

	// Iterate through clients and send their specific view
	for client := range h.Clients {
		var visibleEntities []engine.Entity

		if tank, exists := tankMap[client.TankID]; exists {
			// Filter entities by tank's view range
			visibleEntities = h.Game.GetVisibleEntities(tank.GetPosition(), tank.ViewRange)
		} else {
			visibleEntities = []engine.Entity{}
		}

		state, err := json.Marshal(visibleEntities)
		if err != nil {
			slog.Error("failed to marshal per-client game state", "error", err)
			continue
		}

		select {
		case client.Send <- state:
		default:
			close(client.Send)
			delete(h.Clients, client)
		}
	}
}
