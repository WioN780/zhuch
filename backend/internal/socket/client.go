package socket

import (
	"encoding/json"
	"log/slog"
	"zhuch/pkg/engine"

	"github.com/gorilla/websocket"
)

type Client struct {
	Hub        *Hub
	Conn       *websocket.Conn //connection
	Send       chan []byte
	ClientName string
	TankID     string
}

// This handles messages from the user like WASD and mouse
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
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
			slog.Warn("failed to parse player input", "error", err, "name", c.ClientName, "raw", string(message))
			continue
		}

		// Update the tank in the engine
		h := c.Hub
		h.mu.Lock()
		for _, e := range h.Game.Entities {
			if e.GetID() == c.TankID {
				if tank, ok := e.(*engine.Tank); ok {
					// Always update movement and orientation if provided
					tank.InputVector = input.InputVector
					tank.Orientation = input.Orientation

					// If Type is "fire", perform fire action
					if input.Type == "fire" {
						bullet := tank.Fire(input.Orientation, h.Game.CurrentTick)
						if bullet != nil {
							h.Game.Entities = append(h.Game.Entities, bullet)
						}
					}
				}
				break
			}
		}
		h.mu.Unlock()
	}
}

func (c *Client) WritePump() {
	for message := range c.Send {
		err := c.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			slog.Warn("websocket write error", "error", err, "name", c.ClientName)
			return
		}
	}
}
