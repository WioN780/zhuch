package main

import (
	"log/slog"
	"net/http"
	"os"

	"zhuch/internal/metrics"
	"zhuch/internal/socket"
	"zhuch/pkg/engine"
)

func corsMiddleware(next http.Handler) http.Handler {
	// ALLOWED_ORIGINS is a comma-separated list, e.g. "https://example.com,https://www.example.com".
	// Falls back to "*" when unset (local dev).
	allowed := os.Getenv("ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowed)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
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

	mux := http.NewServeMux()
	mux.HandleFunc("/rooms", ctrl.HandleListRooms)
	mux.HandleFunc("/create", ctrl.HandleCreate)
	mux.HandleFunc("/delete", ctrl.HandleDelete)
	mux.HandleFunc("/ws", ctrl.HandleWebSocket)
	mux.Handle("/metrics", metrics.Handler())

	addr := "0.0.0.0:" + port
	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, corsMiddleware(mux)); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
