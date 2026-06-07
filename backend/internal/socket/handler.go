package socket

import (
	"net/http"
	"zhuch/pkg/engine"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// PlayerInput defines the structure of messages coming from the client
type PlayerInput struct {
	Type        string         `json:"type"` // "input" or "fire"
	InputVector engine.Vector2 `json:"input_vector"`
	MousePos    engine.Vector2 `json:"mouse_pos"`
	Orientation float64        `json:"orientation"`
}
