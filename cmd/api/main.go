package main

import (
	"context"
	"log"

	"github.com/rgarcia2304/flight-circle-search/internal/api"
	"github.com/rgarcia2304/flight-circle-search/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	srv, err := api.NewServer(*cfg)
	if err != nil {
		log.Fatalf("build server: %v", err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			log.Printf("close server: %v", err)
		}
	}()

	log.Printf("api listening on %s", cfg.APIAddr)
	if err := srv.Listen(context.Background()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
