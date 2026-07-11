package main

import (
	"log/slog"
	"net/http"
	"os"

	"zhuch/internal/metrics"
	"zhuch/internal/socket"
	"zhuch/pkg/engine"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	manager := socket.NewManager()

	// Default rooms at startup: a plain FFA room, and a practice room so the
	// demo always has bots to play against without needing /create first.
	manager.CreateRoom("default", engine.DefaultConfig(), "ffa", "champion", 0)
	manager.CreateRoom("practice", engine.DefaultConfig(), "practice", "champion", 0)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctrl := socket.NewRoomController(manager)

	// The frontend is served from a different origin (Vite dev / static host),
	// so the REST endpoints need CORS; websockets are exempt by spec.
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h(w, r)
		}
	}

	http.HandleFunc("/rooms", cors(ctrl.HandleListRooms))
	http.HandleFunc("/create", cors(ctrl.HandleCreate))
	http.HandleFunc("/ws", ctrl.HandleWebSocket)
	http.Handle("/metrics", metrics.Handler())

	// for cloud
	addr := "0.0.0.0:" + port

	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
