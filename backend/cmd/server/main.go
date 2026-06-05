package main

import (
	"log/slog"
	"net/http"
	"os"
	"zhuch/internal/socket"
	"zhuch/pkg/engine"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	manager := socket.NewManager()

	// Create a default room at startup
	manager.CreateRoom("default", engine.DefaultConfig())

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		socket.ServeWs(manager, w, r)
	})

	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		socket.HandleCreateRoom(manager, w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// for cloud
	addr := "0.0.0.0:" + port

	// addr := "localhost:" + port

	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
