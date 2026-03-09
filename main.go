package main

import (
	"embed"
	"flag"
	"log"

	"web-terminal/internal/auth"
	"web-terminal/internal/config"
	"web-terminal/internal/server"
)

//go:embed web/*
var webContent embed.FS

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	authSvc, err := auth.New(&cfg.Auth, "data.db")
	if err != nil {
		log.Fatalf("Failed to init auth: %v", err)
	}
	defer authSvc.Close()

	srv, err := server.New(cfg, authSvc, webContent)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Web Terminal starting on port %d", cfg.Server.Port)
	log.Printf("Shell: %s", cfg.Terminal.Shell)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
