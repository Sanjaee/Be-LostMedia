package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"yourapp/internal/app"
	"yourapp/internal/config"
)

func init() {
	// Semua log hanya ke file → Promtail → Loki → Grafana (tidak ke docker logs)
	logDir := "/var/log/app"
	if err := os.MkdirAll(logDir, 0755); err == nil {
		f, err := os.OpenFile(filepath.Join(logDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			log.SetOutput(f)
		}
	}
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize router
	router := app.NewRouter(cfg)

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

