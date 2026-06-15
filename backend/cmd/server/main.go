package main

import (
	"log/slog"
	"net/http"
	"os"
	"zhuch/internal/socket"
	"zhuch/pkg/engine"
)

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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
	manager.CreateRoom("default", engine.DefaultConfig())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctrl := socket.NewRoomController(manager)

	mux := http.NewServeMux()
	mux.HandleFunc("/rooms", ctrl.HandleListRooms)
	mux.HandleFunc("/create", ctrl.HandleCreate)
	mux.HandleFunc("/ws", ctrl.HandleWebSocket)

	addr := "0.0.0.0:" + port
	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, corsMiddleware(mux)); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
