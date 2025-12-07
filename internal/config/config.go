package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	LLMApiKey   string
	LLMApiURL   string
	LLMModel    string
	TargetURLs  []string
	GithubToken string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local dev)
	_ = godotenv.Load()

	cfg := &Config{
		LLMApiKey:   os.Getenv("LLM_API_KEY"),
		LLMApiURL:   os.Getenv("LLM_API_URL"),
		LLMModel:    os.Getenv("LLM_MODEL"),
		GithubToken: os.Getenv("GITHUB_TOKEN"),
	}

	// Parse CSV for request URLs
	urls := os.Getenv("TARGET_URLS")
	if urls != "" {
		cfg.TargetURLs = strings.Split(urls, ",")
		for i := range cfg.TargetURLs {
			cfg.TargetURLs[i] = strings.TrimSpace(cfg.TargetURLs[i])
		}
	}

	return cfg, nil
}
