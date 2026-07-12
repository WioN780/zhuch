package socket

import (
	"net/http"
	"os"
	"strings"
	"zhuch/pkg/engine"

	"github.com/gorilla/websocket"
)

// allowedOrigins is populated from ALLOWED_ORIGINS env var (comma-separated).
// Empty means all origins are permitted (suitable for local dev).
var allowedOrigins []string

func init() {
	if raw := os.Getenv("ALLOWED_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		if len(allowedOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		for _, o := range allowedOrigins {
			if o == origin {
				return true
			}
		}
		return false
	},
}

// PlayerInput defines the structure of messages coming from the client
type PlayerInput struct {
	Type        string         `json:"type"` // "input" or "fire"
	InputVector engine.Vector2 `json:"input_vector"`
	MousePos    engine.Vector2 `json:"mouse_pos"`
	Orientation float64        `json:"orientation"`
}
