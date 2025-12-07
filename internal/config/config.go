package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	LLMCrawlerApiKey  string
	LLMAnalyzerApiKey string
	LLMCrawlerApiURL  string
	LLMAnalyzerApiURL string
	LLMCrawlerModel   string
	LLMAnalyzerModel  string
	OutputLanguage    string
	TargetURLs        []string
	GithubToken       string
}

func LoadConfig() (*Config, error) {
	// Load .env file if it exists (for local dev)
	_ = godotenv.Load()

	defaultKey := os.Getenv("LLM_API_KEY")

	cfg := &Config{
		LLMCrawlerApiKey:  os.Getenv("LLM_CRAWLER_API_KEY"),
		LLMAnalyzerApiKey: os.Getenv("LLM_ANALYZER_API_KEY"),
		LLMCrawlerApiURL:  os.Getenv("LLM_CRAWLER_API_URL"),
		LLMAnalyzerApiURL: os.Getenv("LLM_ANALYZER_API_URL"),
		LLMCrawlerModel:   os.Getenv("LLM_CRAWLER_MODEL"),
		LLMAnalyzerModel:  os.Getenv("LLM_ANALYZER_MODEL"),
		OutputLanguage:    os.Getenv("OUTPUT_LANGUAGE"),
		GithubToken:       os.Getenv("GITHUB_TOKEN"),
	}

	// Fallback for Keys
	if cfg.LLMCrawlerApiKey == "" {
		cfg.LLMCrawlerApiKey = defaultKey
	}
	if cfg.LLMAnalyzerApiKey == "" {
		cfg.LLMAnalyzerApiKey = defaultKey
	}

	// Fallback for URLs
	defaultURL := os.Getenv("LLM_API_URL")
	if cfg.LLMCrawlerApiURL == "" {
		cfg.LLMCrawlerApiURL = defaultURL
	}
	if cfg.LLMAnalyzerApiURL == "" {
		cfg.LLMAnalyzerApiURL = defaultURL
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
