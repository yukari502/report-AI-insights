package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	LLMApiKey        string
	LLMApiURL        string
	LLMCrawlerModel  string
	LLMAnalyzerModel string
	OutputLanguage   string
	TargetURLs       []string
	GithubToken      string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local dev)
	_ = godotenv.Load()

	cfg := &Config{
		LLMApiKey:        os.Getenv("LLM_API_KEY"),
		LLMApiURL:        os.Getenv("LLM_API_URL"),
		LLMCrawlerModel:  os.Getenv("LLM_CRAWLER_MODEL"),
		LLMAnalyzerModel: os.Getenv("LLM_ANALYZER_MODEL"),
		OutputLanguage:   os.Getenv("OUTPUT_LANGUAGE"),
		GithubToken:      os.Getenv("GITHUB_TOKEN"),
	}

	if cfg.OutputLanguage == "" {
		cfg.OutputLanguage = "Chinese" // Default
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
