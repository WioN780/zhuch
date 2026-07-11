package socket

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	internalbots "zhuch/internal/bots"
	"zhuch/pkg/engine"

	"github.com/gorilla/websocket"
)

type RoomController struct {
	Manager *Manager
}

func NewRoomController(m *Manager) *RoomController {
	return &RoomController{Manager: m}
}

// RoomSummary is the public shape of a room. The Room struct itself is not
// serializable (Hub.Clients has pointer keys) and would leak game internals.
type RoomSummary struct {
	ID      string `json:"id"`
	Mode    string `json:"mode"`
	Players int    `json:"players"`
	Bots    int    `json:"bots"`
	TPS     int    `json:"tps"`
}

// API: GET /rooms
func (c *RoomController) HandleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms := c.Manager.ListRooms()

	summaries := make([]RoomSummary, 0, len(rooms))
	for _, room := range rooms {
		summaries = append(summaries, RoomSummary{
			ID:      room.ID,
			Mode:    string(room.Bots.Mode),
			Players: room.Hub.ClientCount(),
			Bots:    room.Bots.BotCount(),
			TPS:     room.Game.Config.TicksPerSecond,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// protectedRooms are seeded at startup and always available; deleting them
// would leave the public demo without a guaranteed room.
var protectedRooms = map[string]bool{"default": true, "practice": true}

// API: POST /delete
func (c *RoomController) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}
	if protectedRooms[req.ID] {
		http.Error(w, "Cannot delete a built-in room", http.StatusForbidden)
		return
	}
	if !c.Manager.RemoveRoom(req.ID) {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	slog.Info("room deleted", "id", req.ID)
	w.WriteHeader(http.StatusNoContent)
}

// API: POST /create
func (c *RoomController) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string            `json:"id"`
		Config   engine.GameConfig `json:"config"`
		Mode     string            `json:"mode"`
		BotModel string            `json:"bot_model"`
		BotCount int               `json:"bot_count"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "ffa"
	}
	if !internalbots.ValidMode(mode) {
		http.Error(w, "Invalid mode: must be one of ffa, zombies, boss, practice", http.StatusBadRequest)
		return
	}
	botModel := req.BotModel
	if botModel == "" {
		botModel = "champion"
	}

	config := req.Config
	if config.TicksPerSecond == 0 {
		config = engine.DefaultConfig()
	}

	room := c.Manager.CreateRoom(req.ID, config, mode, botModel, req.BotCount)
	if room == nil {
		http.Error(w, "Room already exists", http.StatusConflict)
		return
	}

	slog.Info("room created", "id", req.ID, "mode", mode, "bot_model", botModel)
	w.WriteHeader(http.StatusCreated)
}

// API: GET /ws (The WebSocket Upgrader)
func (c *RoomController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")

	if roomID == "" {
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}

	room := c.Manager.GetRoom(roomID)
	if room == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err, "remote_addr", r.RemoteAddr)
		return
	}

	// Wait for the first message which should be the player name
	_, nameBytes, err := conn.ReadMessage()
	if err != nil {
		slog.Warn("handshake failed: could not read name", "error", err, "remote_addr", r.RemoteAddr)
		conn.Close()
		return
	}
	playerName := strings.TrimSpace(string(nameBytes))

	if playerName == "" {
		slog.Warn("connection rejected: empty name", "remote_addr", r.RemoteAddr)
		errMsg, _ := json.Marshal(map[string]string{"type": "error", "message": "Name cannot be empty"})
		conn.WriteMessage(websocket.TextMessage, errMsg)
		conn.Close()
		return
	}

	if room.Hub.isNameTaken(playerName) {
		slog.Warn("connection rejected: name taken", "name", playerName)
		errMsg, _ := json.Marshal(map[string]string{"type": "error", "message": "Name already in use"})
		conn.WriteMessage(websocket.TextMessage, errMsg)
		conn.Close()
		return
	}

	slog.Info("player handshake successful", "name", playerName, "room", roomID, "remote_addr", r.RemoteAddr)

	// Create the client
	client := &Client{
		Hub:        room.Hub,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		ClientName: playerName,
	}

	// Register with the Hub
	room.Hub.Register <- client

	// Start the processing loops
	go client.WritePump()
	go client.ReadPump()
}
