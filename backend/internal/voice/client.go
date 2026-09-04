package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	host string
	http *http.Client
}

func New(host string) *Client {
	return &Client{host: host, http: &http.Client{Timeout: 30 * time.Minute}}
}

type Status struct {
	Trained     bool     `json:"trained"`
	VocabSize   int      `json:"vocab_size"`
	Params      int64    `json:"params"`
	Steps       int      `json:"steps"`
	ValLoss     float64  `json:"val_loss"`
	TrainedAt   string   `json:"trained_at"`
	CorpusChars int      `json:"corpus_chars"`
	Config      any      `json:"config"`
	Notes       []string `json:"notes"`
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body *bytes.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.host+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("voice service unreachable at %s: %w", c.host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Detail != "" {
			return fmt.Errorf("voice service: %s", e.Detail)
		}
		return fmt.Errorf("voice service status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", nil, nil)
}

func (c *Client) Status(ctx context.Context, user string) (Status, error) {
	var s Status
	err := c.do(ctx, http.MethodGet, "/status?user="+url.QueryEscape(user), nil, &s)
	return s, err
}

type scoreRequest struct {
	User       string   `json:"user"`
	Context    string   `json:"context"`
	Candidates []string `json:"candidates"`
}

type ScoredCandidate struct {
	Text    string  `json:"text"`
	Score   float64 `json:"score"`
	Perplex float64 `json:"perplexity"`
}

type scoreResponse struct {
	Ranked []ScoredCandidate `json:"ranked"`
	Best   string            `json:"best"`
}

func (c *Client) Score(ctx context.Context, user, contextText string, candidates []string) (scoreResponse, error) {
	var r scoreResponse
	err := c.do(ctx, http.MethodPost, "/score", scoreRequest{User: user, Context: contextText, Candidates: candidates}, &r)
	return r, err
}

type rewriteRequest struct {
	User        string  `json:"user"`
	Draft       string  `json:"draft"`
	Context     string  `json:"context"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type rewriteResponse struct {
	Text string `json:"text"`
}

func (c *Client) Rewrite(ctx context.Context, user, draft, contextText string) (string, error) {
	var r rewriteResponse
	err := c.do(ctx, http.MethodPost, "/rewrite", rewriteRequest{
		User: user, Draft: draft, Context: contextText, Temperature: 0.8, MaxTokens: 400,
	}, &r)
	return r.Text, err
}

type generateRequest struct {
	User        string  `json:"user"`
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	TopK        int     `json:"top_k"`
}

type generateResponse struct {
	Text       string `json:"text"`
	Completion string `json:"completion"`
}

func (c *Client) Generate(ctx context.Context, user, prompt string, maxTokens int, temperature float64, topK int) (generateResponse, error) {
	var r generateResponse
	err := c.do(ctx, http.MethodPost, "/generate", generateRequest{
		User: user, Prompt: prompt, MaxTokens: maxTokens, Temperature: temperature, TopK: topK,
	}, &r)
	return r, err
}

type trainRequest struct {
	User       string `json:"user"`
	CorpusPath string `json:"corpus_path"`
	Steps      int    `json:"steps"`
}

type TrainResult struct {
	Status    string  `json:"status"`
	Steps     int     `json:"steps"`
	ValLoss   float64 `json:"val_loss"`
	VocabSize int     `json:"vocab_size"`
	Params    int64   `json:"params"`
	Seconds   float64 `json:"seconds"`
}

func (c *Client) Train(ctx context.Context, user, corpusPath string, steps int) (TrainResult, error) {
	var r TrainResult
	err := c.do(ctx, http.MethodPost, "/train", trainRequest{User: user, CorpusPath: corpusPath, Steps: steps}, &r)
	return r, err
}
