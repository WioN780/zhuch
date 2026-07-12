package socket

import (
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	internalbots "zhuch/internal/bots"
	"zhuch/internal/metrics"
	"zhuch/internal/telemetry"
	"zhuch/pkg/engine"
)

// One producer for the whole server; inert unless KAFKA_BROKERS is set.
var producer = telemetry.New("live")

type Room struct {
	ID     string
	Game   *engine.Game
	Hub    *Hub
	Bots   *internalbots.BotSet
	StopCh chan struct{}
}

// NewRoom builds a room. mode/botModel/botCount configure its BotSet
// ("ffa"/"" = no bots); see internal/bots.NewBotSet.
func NewRoom(id string, config engine.GameConfig, mode, botModel string, botCount int) *Room {
	game := engine.NewGame(config)
	return &Room{
		ID:     id,
		Game:   game,
		Hub:    NewHub(),
		Bots:   internalbots.NewBotSet(mode, botCount, botModel),
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

	r.Game.OnEvent = func(eventType, actor, target string, pos engine.Vector2) {
		producer.Emit(telemetry.Event{
			Room: r.ID, Type: eventType, Actor: actor, Target: target,
			Pos: &telemetry.Pos{X: pos.X, Y: pos.Y},
		})
	}

	ticker := time.NewTicker(time.Second / time.Duration(r.Game.Config.TicksPerSecond))
	defer ticker.Stop()

	slog.Info("room started", "id", r.ID, "tps", r.Game.Config.TicksPerSecond, "mode", r.Bots.Mode)

	for {
		select {
		case <-ticker.C:
			r.drainInputs()
			// Bots decide and act BEFORE Tick, synchronously on this same
			// goroutine — no locking needed. Same assumption as
			// pkg/bots/obs.go's BuildObservation: never call Bots.Act
			// concurrently with Game.Tick.
			r.Bots.Act(r.Game, r.Game.CurrentTick)
			r.Game.Tick()
			r.broadcastState(clients, grace)

			metrics.TickDuration.WithLabelValues(r.ID).Observe(r.Game.Metrics.TickDuration.Seconds())
			metrics.Entities.WithLabelValues(r.ID).Set(float64(r.Game.Metrics.EntityCount))
			metrics.Players.WithLabelValues(r.ID).Set(float64(r.Hub.PlayerCount()))
			metrics.Bots.WithLabelValues(r.ID).Set(float64(r.Bots.BotCount()))

			// 1Hz position samples per tank (contracts §6).
			if r.Game.CurrentTick%20 == 0 {
				for _, t := range r.Game.Tanks() {
					pos := t.GetPosition()
					producer.Emit(telemetry.Event{
						Room: r.ID, Type: "pos_sample", Actor: t.GetID(),
						Pos: &telemetry.Pos{X: pos.X, Y: pos.Y},
					})
				}
			}

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
			// Kick everyone still connected: the room goroutine is exiting,
			// so no one is left to process a normal Unregister — close
			// directly instead (mirrors the pre-refactor Hub.Quit kick).
			for client := range clients {
				client.closeOnce()
				client.Conn.Close()
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
			tank := engine.NewTank(client.ClientName, r.Game.SafeSpawnPos(), &r.Game.Config)
			client.TankID = tank.GetID()
			r.Game.Entities = append(r.Game.Entities, tank)
			slog.Info("client reconnected, tank gone, new spawn", "name", client.ClientName, "tank_id", client.TankID)
		}
	} else {
		tank := engine.NewTank(client.ClientName, r.Game.SafeSpawnPos(), &r.Game.Config)
		client.TankID = tank.GetID()
		r.Game.Entities = append(r.Game.Entities, tank)
		slog.Info("client registered", "name", client.ClientName, "tank_id", client.TankID, "total_clients", len(clients))
	}

	clients[client] = true
	r.Hub.claimName(client.ClientName)

	// Obstacles are static per room, so this is the only time they're sent.
	initMsg, _ := json.Marshal(map[string]any{
		"type":      "init",
		"tank_id":   client.TankID,
		"config":    r.Game.Config,
		"obstacles": obstaclePayload(r.Game.Arena.Obstacles),
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

// CreateRoom builds and starts a new room. mode/botModel/botCount configure
// its bots ("ffa"/"" botModel/count are ignored -> no bots).
func (m *Manager) CreateRoom(id string, config engine.GameConfig, mode, botModel string, botCount int) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Rooms[id]; exists {
		return nil
	}

	room := NewRoom(id, config, mode, botModel, botCount)
	m.Rooms[id] = room
	metrics.Rooms.Set(float64(len(m.Rooms)))
	go room.Start()
	return room
}

func (m *Manager) GetRoom(id string) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Rooms[id]
}

func (m *Manager) ListRooms() []*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return slices.Collect(maps.Values(m.Rooms))
}

// RemoveRoom stops the room's tick loop, kicks every connected client and
// forgets the room. Reports whether the room existed.
func (m *Manager) RemoveRoom(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.Rooms[id]
	if !exists {
		return false
	}
	close(room.StopCh)
	delete(m.Rooms, id)
	metrics.Rooms.Set(float64(len(m.Rooms)))
	return true
}
