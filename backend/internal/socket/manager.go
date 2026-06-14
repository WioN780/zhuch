package socket

import (
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
	"zhuch/pkg/engine"
)

type Room struct {
	ID     string
	Game   *engine.Game
	Hub    *Hub
	StopCh chan struct{}
}

func NewRoom(id string, config engine.GameConfig) *Room {
	game := engine.NewGame(config)
	return &Room{
		ID:     id,
		Game:   game,
		Hub:    NewHub(),
		StopCh: make(chan struct{}),
	}
}

// Start is the room's single goroutine. It is the ONLY goroutine that reads
// or writes game state. All external goroutines communicate via Hub channels.
func (r *Room) Start() {
	clients := make(map[*Client]bool)

	ticker := time.NewTicker(time.Second / time.Duration(r.Game.Config.TicksPerSecond))
	defer ticker.Stop()

	slog.Info("room started", "id", r.ID, "tps", r.Game.Config.TicksPerSecond)

	for {
		select {
		case <-ticker.C:
			r.drainInputs()
			r.Game.Tick()
			r.broadcastState(clients)

		case client := <-r.Hub.Register:
			r.registerClient(clients, client)

		case client := <-r.Hub.Unregister:
			r.unregisterClient(clients, client)

		case <-r.StopCh:
			slog.Info("room stopping", "id", r.ID)
			return
		}
	}
}

// drainInputs applies all buffered player inputs before the next tick.
func (r *Room) drainInputs() {
	for {
		select {
		case evt := <-r.Hub.InputCh:
			for _, e := range r.Game.Entities {
				if e.GetID() != evt.TankID {
					continue
				}
				tank, ok := e.(*engine.Tank)
				if !ok {
					break
				}
				tank.InputVector = evt.Input.InputVector
				tank.Orientation = evt.Input.Orientation
				if evt.Input.Type == "fire" {
					if bullet := tank.Fire(evt.Input.Orientation, r.Game.CurrentTick); bullet != nil {
						r.Game.Entities = append(r.Game.Entities, bullet)
					}
				}
				break
			}
		default:
			return
		}
	}
}

func (r *Room) registerClient(clients map[*Client]bool, client *Client) {
	tank := engine.NewTank(client.ClientName, engine.Vector2{X: 100, Y: 100}, &r.Game.Config)
	client.TankID = tank.GetID()
	r.Game.Entities = append(r.Game.Entities, tank)
	clients[client] = true
	r.Hub.claimName(client.ClientName)

	initMsg, _ := json.Marshal(map[string]any{
		"type":    "init",
		"tank_id": client.TankID,
		"config":  r.Game.Config,
	})
	safeSend(client.Send, initMsg)
	slog.Info("client registered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(clients))
}

func (r *Room) unregisterClient(clients map[*Client]bool, client *Client) {
	if _, ok := clients[client]; !ok {
		return
	}
	delete(clients, client)
	r.Hub.releaseName(client.ClientName)
	client.closeOnce()

	newEntities := r.Game.Entities[:0]
	for _, e := range r.Game.Entities {
		if e.GetID() != client.TankID {
			newEntities = append(newEntities, e)
		}
	}
	for i := len(newEntities); i < len(r.Game.Entities); i++ {
		r.Game.Entities[i] = nil
	}
	r.Game.Entities = newEntities
	slog.Info("client unregistered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(clients))
}

func (r *Room) broadcastState(clients map[*Client]bool) {
	tankMap := make(map[string]*engine.Tank)
	for _, e := range r.Game.Entities {
		if t, ok := e.(*engine.Tank); ok {
			tankMap[t.GetID()] = t
		}
	}

	for client := range clients {
		var visibleEntities []engine.Entity
		isDead := false

		if tank, exists := tankMap[client.TankID]; exists {
			visibleEntities = r.Game.GetVisibleEntities(tank.GetPosition(), tank.ViewRange)
		} else {
			visibleEntities = []engine.Entity{}
			isDead = true
		}

		packet := map[string]interface{}{
			"entities": visibleEntities,
			"metrics":  r.Game.Metrics,
		}
		if isDead {
			packet["type"] = "dead"
		}

		state, err := json.Marshal(packet)
		if err != nil {
			slog.Error("failed to marshal game state", "error", err)
			continue
		}

		if !safeSend(client.Send, state) {
			slog.Warn("client send buffer full, dropping", "name", client.ClientName)
			r.unregisterClient(clients, client)
		}
	}
}

// ----- Manager -----

type Manager struct {
	Rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{Rooms: make(map[string]*Room)}
}

func (m *Manager) CreateRoom(id string, config engine.GameConfig) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Rooms[id]; exists {
		return nil
	}

	room := NewRoom(id, config)
	m.Rooms[id] = room
	go room.Start()
	return room
}

func (m *Manager) GetRoom(id string) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Rooms[id]
}

func (m *Manager) ListRooms() []*Room {
	return slices.Collect(maps.Values(m.Rooms))
}

func (m *Manager) RemoveRoom(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, exists := m.Rooms[id]; exists {
		close(room.StopCh)
		delete(m.Rooms, id)
	}
}
