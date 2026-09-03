package server

import (
	"encoding/json"
	"net/http"
	"time"

	"llamaweb/internal/agent"
	"llamaweb/internal/store"
)

func (s *Server) handleOllamaVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": "llamaweb-1.0"})
}

func (s *Server) handleOllamaTags(w http.ResponseWriter, r *http.Request) {
	models, err := s.llm.Tags(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

type ollamaGenerateReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream *bool  `json:"stream"`
	Voice  bool   `json:"voice"`
}

func (s *Server) handleOllamaGenerate(w http.ResponseWriter, r *http.Request) {
	var req ollamaGenerateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	stream := req.Stream == nil || *req.Stream
	model := req.Model
	if model == "" {
		model = s.cfg.OllamaModel
	}

	var history []store.Message
	if req.System != "" {
		history = append(history, store.Message{Role: "system", Content: req.System})
	}

	if !stream {
		res, err := s.agent.Respond(r.Context(), model, history, req.Prompt, req.Voice, agent.Events{})
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": model, "created_at": nowRFC(), "response": res.Text, "done": true,
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	ev := agent.Events{OnToken: func(t string) {
		_ = enc.Encode(map[string]any{"model": model, "created_at": nowRFC(), "response": t, "done": false})
		flusher.Flush()
	}}
	res, err := s.agent.Respond(r.Context(), model, history, req.Prompt, req.Voice, ev)
	if err != nil {
		_ = enc.Encode(map[string]any{"model": model, "created_at": nowRFC(), "response": "", "done": true, "error": err.Error()})
		flusher.Flush()
		return
	}
	_ = enc.Encode(map[string]any{
		"model": model, "created_at": nowRFC(), "response": "", "done": true,
		"done_reason": "stop", "total_duration": 0, "voiced": res.Voiced,
	})
	flusher.Flush()
}

type ollamaChatReq struct {
	Model    string          `json:"model"`
	Messages []store.Message `json:"messages"`
	Stream   *bool           `json:"stream"`
	Voice    bool            `json:"voice"`
}

func (s *Server) handleOllamaChat(w http.ResponseWriter, r *http.Request) {
	var req ollamaChatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if len(req.Messages) == 0 {
		writeErr(w, http.StatusBadRequest, "messages required")
		return
	}
	stream := req.Stream == nil || *req.Stream
	model := req.Model
	if model == "" {
		model = s.cfg.OllamaModel
	}

	history := req.Messages[:len(req.Messages)-1]
	last := req.Messages[len(req.Messages)-1]
	userMsg := last.Content
	if last.Role != "user" {
		userMsg = last.Role + ": " + last.Content
	}

	if !stream {
		res, err := s.agent.Respond(r.Context(), model, history, userMsg, req.Voice, agent.Events{})
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": model, "created_at": nowRFC(),
			"message": map[string]string{"role": "assistant", "content": res.Text},
			"done":    true,
		})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	enc := json.NewEncoder(w)
	ev := agent.Events{OnToken: func(t string) {
		_ = enc.Encode(map[string]any{
			"model": model, "created_at": nowRFC(),
			"message": map[string]string{"role": "assistant", "content": t},
			"done":    false,
		})
		flusher.Flush()
	}}
	res, err := s.agent.Respond(r.Context(), model, history, userMsg, req.Voice, ev)
	if err != nil {
		_ = enc.Encode(map[string]any{
			"model": model, "created_at": nowRFC(),
			"message": map[string]string{"role": "assistant", "content": ""},
			"done":    true, "error": err.Error(),
		})
		flusher.Flush()
		return
	}
	_ = enc.Encode(map[string]any{
		"model": model, "created_at": nowRFC(),
		"message":     map[string]string{"role": "assistant", "content": ""},
		"done":        true,
		"done_reason": "stop",
		"voiced":      res.Voiced,
	})
	flusher.Flush()
}
