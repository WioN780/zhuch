package socket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Hub        *Hub
	Conn       *websocket.Conn
	Send       chan []byte
	ClientName string
	TankID     string
	once       sync.Once
}

// closeOnce closes the Send channel exactly once, safe to call from multiple goroutines.
func (c *Client) closeOnce() {
	c.once.Do(func() { close(c.Send) })
}

// safeSend attempts a non-blocking send to ch. Returns false if full or closed.
func safeSend(ch chan []byte, data []byte) (sent bool) {
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	select {
	case ch <- data:
		return true
	default:
		return false
	}
}

// ReadPump reads messages from the WebSocket and forwards them to the room goroutine
// via Hub.InputCh. It never touches game state directly.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("websocket read error", "error", err, "name", c.ClientName)
			}
			break
		}

		var input PlayerInput
		if err := json.Unmarshal(message, &input); err != nil {
			slog.Warn("failed to parse player input", "error", err, "name", c.ClientName)
			continue
		}

		if input.Type == "ping" {
			pong, _ := json.Marshal(map[string]string{"type": "pong"})
			safeSend(c.Send, pong)
			continue
		}

		// Non-blocking: if the room is overwhelmed, drop the input rather than block.
		select {
		case c.Hub.InputCh <- InputEvent{TankID: c.TankID, Input: input}:
		default:
		}
	}
}

func (c *Client) WritePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			slog.Warn("websocket write error", "error", err, "name", c.ClientName)
			return
		}
	}
}
