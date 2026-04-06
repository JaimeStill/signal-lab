package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JaimeStill/signal-lab/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config load failed:", err)
	}

	srv := NewServer(cfg)

	if err := srv.Start(); err != nil {
		log.Fatal("service start failed:", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	if err := srv.Shutdown(); err != nil {
		log.Fatal("shutdown failed:", err)
	}

	log.Println("sensor stopped gracefully")
}
