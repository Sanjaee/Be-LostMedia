package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"yourapp/internal/app"
	"yourapp/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	// Semua log (termasuk [GIN]) hanya ke file → Promtail → Loki → Grafana (tidak ke docker logs)
	logDir := "/var/log/app"
	if err := os.MkdirAll(logDir, 0755); err == nil {
		f, err := os.OpenFile(filepath.Join(logDir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			log.SetOutput(f)
			gin.DefaultWriter = f
			gin.DefaultErrorWriter = f
		}
	}

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

