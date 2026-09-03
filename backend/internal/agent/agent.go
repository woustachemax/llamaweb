package agent

import (
	"context"
	"strings"
	"sync"

	"llamaweb/internal/config"
	"llamaweb/internal/ollama"
	"llamaweb/internal/store"
	"llamaweb/internal/voice"
)

type Agent struct {
	cfg   config.Config
	llm   *ollama.Client
	voice *voice.Client
}

func New(cfg config.Config, llm *ollama.Client, v *voice.Client) *Agent {
	return &Agent{cfg: cfg, llm: llm, voice: v}
}

type Result struct {
	Text     string
	Original string
	Voiced   bool
	Ranking  []voice.ScoredCandidate
}

type Events struct {
	OnToken func(string)
	OnStage func(string)
}

func (a *Agent) buildMessages(history []store.Message, userMsg string) []ollama.Message {
	msgs := []ollama.Message{{Role: "system", Content: a.cfg.SystemPrompt}}
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, ollama.Message{Role: role, Content: m.Content})
	}
	msgs = append(msgs, ollama.Message{Role: "user", Content: userMsg})
	return msgs
}

func recentContext(history []store.Message, userMsg string) string {
	var b strings.Builder
	start := 0
	if len(history) > 6 {
		start = len(history) - 6
	}
	for _, m := range history[start:] {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	b.WriteString("user: ")
	b.WriteString(userMsg)
	return b.String()
}

func (a *Agent) Respond(ctx context.Context, model string, history []store.Message, userMsg string, useVoice bool, ev Events) (Result, error) {
	if model == "" {
		model = a.cfg.OllamaModel
	}
	msgs := a.buildMessages(history, userMsg)

	voiceReady := false
	if useVoice {
		if st, err := a.voice.Status(ctx); err == nil && st.Trained {
			voiceReady = true
		}
	}

	if !voiceReady {
		if ev.OnStage != nil {
			ev.OnStage("generating")
		}
		text, err := a.llm.Chat(ctx, model, msgs, nil, ev.OnToken)
		if err != nil {
			return Result{}, err
		}
		return Result{Text: text, Original: text}, nil
	}

	switch a.cfg.VoiceMode {
	case "rewrite":
		return a.respondRewrite(ctx, model, msgs, history, userMsg, ev)
	default:
		return a.respondRerank(ctx, model, msgs, history, userMsg, ev)
	}
}

func (a *Agent) respondRerank(ctx context.Context, model string, msgs []ollama.Message, history []store.Message, userMsg string, ev Events) (Result, error) {
	n := a.cfg.VoiceCandidates
	if n < 2 {
		n = 2
	}
	if ev.OnStage != nil {
		ev.OnStage("drafting candidates")
	}
	cands := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			opts := map[string]any{"temperature": 0.6 + 0.25*float64(i)}
			cands[i], errs[i] = a.llm.Complete(ctx, model, msgs, opts)
		}(i)
	}
	wg.Wait()

	var valid []string
	for i, c := range cands {
		if errs[i] == nil && strings.TrimSpace(c) != "" {
			valid = append(valid, c)
		}
	}
	if len(valid) == 0 {
		for _, e := range errs {
			if e != nil {
				return Result{}, e
			}
		}
		return Result{Text: "", Original: ""}, nil
	}

	if ev.OnStage != nil {
		ev.OnStage("matching your voice")
	}
	ranked, err := a.voice.Score(ctx, recentContext(history, userMsg), valid)
	best := valid[0]
	var ranking []voice.ScoredCandidate
	if err == nil && ranked.Best != "" {
		best = ranked.Best
		ranking = ranked.Ranked
	}

	stream(best, ev.OnToken)
	return Result{Text: best, Original: valid[0], Voiced: true, Ranking: ranking}, nil
}

func (a *Agent) respondRewrite(ctx context.Context, model string, msgs []ollama.Message, history []store.Message, userMsg string, ev Events) (Result, error) {
	if ev.OnStage != nil {
		ev.OnStage("drafting")
	}
	draft, err := a.llm.Complete(ctx, model, msgs, nil)
	if err != nil {
		return Result{}, err
	}
	if ev.OnStage != nil {
		ev.OnStage("matching your voice")
	}
	voiced, err := a.voice.Rewrite(ctx, draft, recentContext(history, userMsg))
	if err != nil || strings.TrimSpace(voiced) == "" {
		stream(draft, ev.OnToken)
		return Result{Text: draft, Original: draft}, nil
	}
	stream(voiced, ev.OnToken)
	return Result{Text: voiced, Original: draft, Voiced: true}, nil
}

func stream(text string, onToken func(string)) {
	if onToken == nil {
		return
	}
	fields := chunkText(text)
	for _, f := range fields {
		onToken(f)
	}
}

func chunkText(text string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range text {
		cur.WriteRune(r)
		if r == ' ' || r == '\n' {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
