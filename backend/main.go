package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llamaweb/internal/agent"
	"llamaweb/internal/auth"
	"llamaweb/internal/config"
	"llamaweb/internal/ollama"
	"llamaweb/internal/server"
	"llamaweb/internal/store"
	"llamaweb/internal/voice"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.StatePath, cfg.CorpusDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	au, err := auth.New(cfg.AuthPath)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	llm := ollama.New(cfg.OllamaHost)
	v := voice.New(cfg.MLHost)
	ag := agent.New(cfg, llm, v)
	srv := server.New(cfg, st, au, llm, v, ag)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("llamaweb backend listening on %s", cfg.Addr)
		log.Printf("  ollama:  %s (model %s)", cfg.OllamaHost, cfg.OllamaModel)
		log.Printf("  voice:   %s (mode %s)", cfg.MLHost, cfg.VoiceMode)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	log.Println("shutdown complete")
}
