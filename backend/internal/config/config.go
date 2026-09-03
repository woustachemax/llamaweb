package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr            string
	OllamaHost      string
	OllamaModel     string
	MLHost          string
	DataDir         string
	CorpusPath      string
	StatePath       string
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

func Load() Config {
	dataDir := getenv("LLAMAWEB_DATA_DIR", "./data")
	return Config{
		Addr:            getenv("LLAMAWEB_ADDR", ":8080"),
		OllamaHost:      getenv("OLLAMA_HOST", "http://localhost:11434"),
		OllamaModel:     getenv("OLLAMA_MODEL", "llama3.2"),
		MLHost:          getenv("ML_HOST", "http://localhost:8000"),
		DataDir:         dataDir,
		CorpusPath:      getenv("LLAMAWEB_CORPUS", "../ml/data/user_corpus.txt"),
		StatePath:       getenv("LLAMAWEB_STATE", dataDir+"/state.json"),
		VoiceMode:       getenv("VOICE_MODE", "rerank"),
		VoiceCandidates: getenvInt("VOICE_CANDIDATES", 3),
		SystemPrompt: getenv("LLAMAWEB_SYSTEM_PROMPT",
			"You are a helpful, direct assistant running on a self-hosted web backend. "+
				"Answer clearly and concisely. Match the tone and phrasing habits of the person you are talking to."),
	}
}
