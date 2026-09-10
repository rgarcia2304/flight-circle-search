package main

import (
	"context"
	"log"

	"github.com/rgarcia2304/flight-circle-search/internal/config"
	"github.com/rgarcia2304/flight-circle-search/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	runner, err := worker.NewRunner(*cfg)
	if err != nil {
		log.Fatalf("build worker: %v", err)
	}
	defer func() {
		if err := runner.Close(); err != nil {
			log.Printf("close worker: %v", err)
		}
	}()

	if err := runner.Run(context.Background()); err != nil {
		log.Fatalf("run: %v", err)
	}
}
