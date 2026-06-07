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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctrl := socket.NewRoomController(manager)

	http.HandleFunc("/rooms", ctrl.HandleListRooms)
	http.HandleFunc("/create", ctrl.HandleCreate)
	http.HandleFunc("/ws", ctrl.HandleWebSocket)

	// for cloud
	addr := "0.0.0.0:" + port

	slog.Info("server starting", "addr", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
