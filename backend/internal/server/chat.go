package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"llamaweb/internal/agent"
	"llamaweb/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"status": "ok", "model": s.cfg.OllamaModel}
	if _, err := s.llm.Tags(r.Context()); err != nil {
		out["ollama"] = err.Error()
	} else {
		out["ollama"] = "ok"
	}
	if _, err := s.voice.Status(r.Context()); err != nil {
		out["voice"] = err.Error()
	} else {
		out["voice"] = "ok"
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.llm.Tags(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models, "default": s.cfg.OllamaModel})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, http.StatusOK, s.store.CreateSession(body.Title))
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.store.ListSessions()})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sess, err := s.store.GetSession(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSession(r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type chatBody struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Model     string `json:"model"`
	Voice     bool   `json:"voice"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Message == "" {
		writeErr(w, http.StatusBadRequest, "message is required")
		return
	}
	if body.SessionID == "" {
		body.SessionID = s.store.CreateSession("").ID
	}

	history, err := s.store.History(body.SessionID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "session not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(event string, payload any) {
		data, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	userMsg, err := s.store.AppendMessage(body.SessionID, store.Message{Role: "user", Content: body.Message})
	if err != nil {
		send("error", map[string]string{"message": err.Error()})
		return
	}
	send("session", map[string]string{"session_id": body.SessionID, "user_message_id": userMsg.ID})

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()

	ev := agent.Events{
		OnToken: func(t string) { send("token", map[string]string{"text": t}) },
		OnStage: func(stage string) { send("stage", map[string]string{"stage": stage}) },
	}

	res, err := s.agent.Respond(ctx, body.Model, history, body.Message, body.Voice, ev)
	if err != nil {
		send("error", map[string]string{"message": err.Error()})
		return
	}

	msg := store.Message{Role: "assistant", Content: res.Text, Voiced: res.Voiced}
	if res.Voiced {
		msg.Original = res.Original
	}
	saved, err := s.store.AppendMessage(body.SessionID, msg)
	if err != nil {
		send("error", map[string]string{"message": err.Error()})
		return
	}
	send("done", map[string]any{
		"message_id": saved.ID,
		"text":       res.Text,
		"original":   res.Original,
		"voiced":     res.Voiced,
		"ranking":    res.Ranking,
	})
}

func (s *Server) handleVoiceStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.voice.Status(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleVoiceTrain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Steps int `json:"steps"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := s.voice.Train(r.Context(), s.cfg.CorpusPath, body.Steps)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleVoiceGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt      string  `json:"prompt"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
		TopK        int     `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.MaxTokens == 0 {
		body.MaxTokens = 200
	}
	if body.Temperature == 0 {
		body.Temperature = 0.8
	}
	res, err := s.voice.Generate(r.Context(), body.Prompt, body.MaxTokens, body.Temperature, body.TopK)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
