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

const gracePeriod = 30 * time.Second

type graceEntry struct {
	tankID string
	timer  *time.Timer
}

// Start is the room's single goroutine. It is the ONLY goroutine that reads
// or writes game state. All external goroutines communicate via Hub channels.
func (r *Room) Start() {
	clients := make(map[*Client]bool)
	grace := make(map[string]*graceEntry)

	ticker := time.NewTicker(time.Second / time.Duration(r.Game.Config.TicksPerSecond))
	defer ticker.Stop()

	slog.Info("room started", "id", r.ID, "tps", r.Game.Config.TicksPerSecond)

	for {
		select {
		case <-ticker.C:
			r.drainInputs()
			r.Game.Tick()
			r.broadcastState(clients, grace)

		case client := <-r.Hub.Register:
			r.registerClient(clients, grace, client)

		case client := <-r.Hub.Unregister:
			r.unregisterClient(clients, grace, client)

		case name := <-r.Hub.GraceExpired:
			r.expireGrace(grace, name)

		case <-r.StopCh:
			for _, e := range grace {
				e.timer.Stop()
			}
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

func (r *Room) registerClient(clients map[*Client]bool, grace map[string]*graceEntry, client *Client) {
	if entry, ok := grace[client.ClientName]; ok {
		// Reconnecting player — cancel the grace timer and reuse the tank if still alive.
		entry.timer.Stop()
		delete(grace, client.ClientName)

		tankAlive := false
		for _, e := range r.Game.Entities {
			if e.GetID() == entry.tankID {
				tankAlive = true
				break
			}
		}

		if tankAlive {
			client.TankID = entry.tankID
			slog.Info("client reconnected", "name", client.ClientName, "tank_id", client.TankID)
		} else {
			tank := engine.NewTank(client.ClientName, engine.Vector2{X: 100, Y: 100}, &r.Game.Config)
			client.TankID = tank.GetID()
			r.Game.Entities = append(r.Game.Entities, tank)
			slog.Info("client reconnected, tank gone, new spawn", "name", client.ClientName, "tank_id", client.TankID)
		}
	} else {
		tank := engine.NewTank(client.ClientName, engine.Vector2{X: 100, Y: 100}, &r.Game.Config)
		client.TankID = tank.GetID()
		r.Game.Entities = append(r.Game.Entities, tank)
		slog.Info("client registered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(clients))
	}

	clients[client] = true
	r.Hub.claimName(client.ClientName)

	initMsg, _ := json.Marshal(map[string]any{
		"type":    "init",
		"tank_id": client.TankID,
		"config":  r.Game.Config,
	})
	safeSend(client.Send, initMsg)
}

func (r *Room) unregisterClient(clients map[*Client]bool, grace map[string]*graceEntry, client *Client) {
	if _, ok := clients[client]; !ok {
		return
	}
	delete(clients, client)
	r.Hub.releaseName(client.ClientName)
	client.closeOnce()

	// Zero input so the tank coasts to a stop during the grace window.
	for _, e := range r.Game.Entities {
		if e.GetID() == client.TankID {
			if tank, ok := e.(*engine.Tank); ok {
				tank.InputVector = engine.Vector2{}
			}
			break
		}
	}

	name := client.ClientName
	tankID := client.TankID
	timer := time.AfterFunc(gracePeriod, func() {
		r.Hub.GraceExpired <- name
	})
	grace[name] = &graceEntry{tankID: tankID, timer: timer}
	slog.Info("client grace period started", "name", name, "tank_id", tankID)
}

func (r *Room) expireGrace(grace map[string]*graceEntry, name string) {
	entry, ok := grace[name]
	if !ok {
		return
	}
	delete(grace, name)

	newEntities := r.Game.Entities[:0]
	for _, e := range r.Game.Entities {
		if e.GetID() != entry.tankID {
			newEntities = append(newEntities, e)
		}
	}
	for i := len(newEntities); i < len(r.Game.Entities); i++ {
		r.Game.Entities[i] = nil
	}
	r.Game.Entities = newEntities
	slog.Info("client grace expired, tank removed", "name", name, "tank_id", entry.tankID)
}

func (r *Room) broadcastState(clients map[*Client]bool, grace map[string]*graceEntry) {
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
			r.unregisterClient(clients, grace, client)
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
