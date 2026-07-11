package socket

import (
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	internalbots "zhuch/internal/bots"
	"zhuch/internal/metrics"
	"zhuch/pkg/engine"
)

// manager is for organising games and hubs into rooms

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
	hub := NewHub(game)
	return &Room{
		ID:     id,
		Game:   game,
		Hub:    hub,
		Bots:   internalbots.NewBotSet(mode, botCount, botModel),
		StopCh: make(chan struct{}),
	}
}

func (r *Room) Start() {
	go r.Hub.Run()

	ticker := time.NewTicker(time.Second / time.Duration(r.Game.Config.TicksPerSecond))
	defer ticker.Stop()

	slog.Info("room started", "id", r.ID, "tps", r.Game.Config.TicksPerSecond, "mode", r.Bots.Mode)

	for {
		select {
		case <-ticker.C:
			// Bots decide and act BEFORE Tick, synchronously on this same
			// goroutine — no locking needed. Same assumption as
			// pkg/bots/obs.go's BuildObservation: never call Bots.Act
			// concurrently with Game.Tick.
			r.Bots.Act(r.Game, r.Game.CurrentTick)
			r.Game.Tick()
			r.Hub.BroadcastGameState()

			metrics.TickDuration.WithLabelValues(r.ID).Observe(r.Game.Metrics.TickDuration.Seconds())
			metrics.Entities.WithLabelValues(r.ID).Set(float64(r.Game.Metrics.EntityCount))
			metrics.Players.WithLabelValues(r.ID).Set(float64(r.Hub.ClientCount()))
			metrics.Bots.WithLabelValues(r.ID).Set(float64(r.Bots.BotCount()))
		case <-r.StopCh:
			slog.Info("room stopping", "id", r.ID)
			return
		}
	}
}

type Manager struct {
	Rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		Rooms: make(map[string]*Room),
	}
}

// CreateRoom builds and starts a new room. mode/botModel/botCount configure
// its bots ("ffa"/"" botModel/count are ignored -> no bots).
func (m *Manager) CreateRoom(id string, config engine.GameConfig, mode, botModel string, botCount int) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Rooms[id]; exists {
		return nil // Room already exists
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
	close(room.Hub.Quit)
	delete(m.Rooms, id)
	metrics.Rooms.Set(float64(len(m.Rooms)))
	return true
}
