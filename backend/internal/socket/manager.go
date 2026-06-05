package socket

import (
	"log/slog"
	"sync"
	"time"
	"zhuch/pkg/engine"
)

// manager is for organising games and hubs into rooms

type Room struct {
	ID     string
	Game   *engine.Game
	Hub    *Hub
	StopCh chan struct{}
}

func NewRoom(id string, config engine.GameConfig) *Room {
	game := engine.NewGame(config)
	hub := NewHub(game)
	return &Room{
		ID:     id,
		Game:   game,
		Hub:    hub,
		StopCh: make(chan struct{}),
	}
}

func (r *Room) Start() {
	go r.Hub.Run()

	ticker := time.NewTicker(time.Second / time.Duration(r.Game.Config.TicksPerSecond))
	defer ticker.Stop()

	slog.Info("room started", "id", r.ID, "tps", r.Game.Config.TicksPerSecond)

	for {
		select {
		case <-ticker.C:
			r.Game.Tick()
			r.Hub.BroadcastGameState()
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

func (m *Manager) CreateRoom(id string, config engine.GameConfig) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.Rooms[id]; exists {
		return nil // Room already exists
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

func (m *Manager) RemoveRoom(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, exists := m.Rooms[id]; exists {
		close(room.StopCh)
		delete(m.Rooms, id)
	}
}
