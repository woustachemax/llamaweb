package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"llamaweb/internal/agent"
	"llamaweb/internal/config"
	"llamaweb/internal/ollama"
	"llamaweb/internal/store"
	"llamaweb/internal/voice"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	llm   *ollama.Client
	voice *voice.Client
	agent *agent.Agent
}

func New(cfg config.Config, st *store.Store, llm *ollama.Client, v *voice.Client, ag *agent.Agent) *Server {
	return &Server{cfg: cfg, store: st, llm: llm, voice: v, agent: ag}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /app/health", s.handleHealth)
	mux.HandleFunc("GET /app/models", s.handleModels)
	mux.HandleFunc("POST /app/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /app/sessions", s.handleListSessions)
	mux.HandleFunc("GET /app/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /app/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /app/chat", s.handleChat)
	mux.HandleFunc("GET /app/voice/status", s.handleVoiceStatus)
	mux.HandleFunc("POST /app/voice/train", s.handleVoiceTrain)
	mux.HandleFunc("POST /app/voice/generate", s.handleVoiceGenerate)

	mux.HandleFunc("GET /api/version", s.handleOllamaVersion)
	mux.HandleFunc("GET /api/tags", s.handleOllamaTags)
	mux.HandleFunc("POST /api/generate", s.handleOllamaGenerate)
	mux.HandleFunc("POST /api/chat", s.handleOllamaChat)

	return withCORS(withLogging(mux))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
