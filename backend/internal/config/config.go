package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Addr            string
	OllamaHost      string
	OllamaModel     string
	MLHost          string
	DataDir         string
	CorpusDir       string
	StatePath       string
	AuthPath        string
	CookieSecure    bool
	VoiceMode       string
	VoiceCandidates int
	SystemPrompt    string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func Load() Config {
	dataDir := getenv("LLAMAWEB_DATA_DIR", "./data")
	return Config{
		Addr:            getenv("LLAMAWEB_ADDR", ":8080"),
		OllamaHost:      getenv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:     getenv("OLLAMA_MODEL", "llama3.2"),
		MLHost:          getenv("ML_HOST", "http://localhost:8000"),
		DataDir:         dataDir,
		CorpusDir:       getenv("LLAMAWEB_CORPUS_DIR", "../ml/data"),
		StatePath:       getenv("LLAMAWEB_STATE", dataDir+"/state.json"),
		AuthPath:        getenv("LLAMAWEB_AUTH", dataDir+"/auth.json"),
		CookieSecure:    getenvBool("LLAMAWEB_COOKIE_SECURE", false),
		VoiceMode:       getenv("VOICE_MODE", "rerank"),
		VoiceCandidates: getenvInt("VOICE_CANDIDATES", 3),
		SystemPrompt: getenv("LLAMAWEB_SYSTEM_PROMPT",
			"You are a helpful, direct assistant running on a self-hosted web backend. "+
				"Answer clearly and concisely. Match the tone and phrasing habits of the person you are talking to."),
	}
}

func (c Config) CorpusPathFor(userID string) string {
	return filepath.Join(c.CorpusDir, "corpus_"+userID+".txt")
}
