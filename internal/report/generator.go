package report

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kaze/eco_report/internal/config"
	"github.com/kaze/eco_report/internal/crawler"
	"github.com/kaze/eco_report/internal/history"
	"github.com/kaze/eco_report/internal/llm"
)

type Generator struct {
	cfg         *config.Config
	llmClient   *llm.Client
	history     *history.History
	bankMapping map[string]string
}

func NewGenerator(cfg *config.Config) *Generator {
	hist, _ := history.NewHistory("data/history.json") // Ignoring error for simplicity, will start empty
	g := &Generator{
		cfg:       cfg,
		llmClient: llm.NewClient(cfg),
		history:   hist,
	}
	g.loadBankMapping()
	return g
}

func (g *Generator) loadBankMapping() {
	data, err := os.ReadFile(filepath.Join("data", "banks.json"))
	if err == nil {
		json.Unmarshal(data, &g.bankMapping)
	}
}

// CacheItem represents the raw data stored in cache
type CacheItem struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Source      string `json:"source"`
	FetchedDate string `json:"fetched_date"`
}

// GenerateWeekly orchestrates the full weekly report workflow
func (g *Generator) GenerateWeekly() error {
	if err := g.FetchAll(); err != nil {
		return err
	}
	// Wait a bit or straight to summarize
	log.Println("Crawling complete. Starting summarization...")
	return g.SummarizeAll()
}

// FetchAll crawls target URLs and caches content
func (g *Generator) FetchAll() error {
	var wg sync.WaitGroup
	timestamp := time.Now().Format("2006-01-02")

	cacheDir := filepath.Join("data", "cache", timestamp)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	for _, indexURL := range g.cfg.TargetURLs {
		if indexURL == "" {
			continue
		}
		wg.Add(1)
		go func(targetIndexURL string) {
			defer wg.Done()

			log.Printf("AI Crawling Index: %s", targetIndexURL)

			rawHTML, err := crawler.FetchRawHTML(targetIndexURL)
			if err != nil {
				log.Printf("Failed to fetch index %s: %v", targetIndexURL, err)
				return
			}

			log.Printf("Analyzing index with AI...")
			linksJSON, err := g.llmClient.ExtractLinks(rawHTML, timestamp)
			if err != nil {
				log.Printf("LLM extraction failed for %s: %v", targetIndexURL, err)
				return
			}

			linksJSON = cleanJSON(linksJSON)
			var articleLinks []string
			if err := json.Unmarshal([]byte(linksJSON), &articleLinks); err != nil {
				log.Printf("Failed to parse links JSON for %s: %v", targetIndexURL, err)
				return
			}

			log.Printf("Found %d potential articles from %s", len(articleLinks), targetIndexURL)

			for _, link := range articleLinks {
				if !strings.HasPrefix(link, "http") {
					continue
				}

				safeName := sanitizeFilename(link)
				cacheFile := filepath.Join(cacheDir, safeName+".json")

				// Check History & Cache
				if g.history.HasCrawled(link) {
					log.Printf("Already Crawled (History): %s", link)
					continue
				}
				if _, err := os.Stat(cacheFile); err == nil {
					log.Printf("Hit Cache (File): %s", link)
					g.history.AddCrawled(link) // Sync history if file exists
					continue
				}

				// Fetch & Cache
				log.Printf("Fetching: %s", link)
				article, err := crawler.FetchContent(link)
				if err != nil {
					log.Printf("Error crawling %s: %v", link, err)
					continue
				}

				cacheItem := CacheItem{
					URL:         link,
					Title:       article.Title,
					Content:     article.Content,
					Source:      article.Source,
					FetchedDate: timestamp,
				}

				data, _ := json.MarshalIndent(cacheItem, "", "  ")
				os.WriteFile(cacheFile, data, 0644)
				g.history.AddCrawled(link)
			}
		}(indexURL)
	}

	wg.Wait()
	return nil
}

// SummarizeAll processes cached articles and generates reports
func (g *Generator) SummarizeAll() error {
	timestamp := time.Now().Format("2006-01-02")
	cacheDir := filepath.Join("data", "cache", timestamp)
	postsBaseDir := filepath.Join("data", "posts")

	if err := os.MkdirAll(postsBaseDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(postsBaseDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No cache found for today. Run Fetch first.")
			return nil
		}
		return err
	}

	log.Printf("Found %d cached items to process...", len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		cachePath := filepath.Join(cacheDir, entry.Name())
		data, err := os.ReadFile(cachePath)
		if err != nil {
			continue
		}

		var cacheItem CacheItem
		if err := json.Unmarshal(data, &cacheItem); err != nil {
			log.Printf("Invalid cache file %s: %v", entry.Name(), err)
			continue
		}

		if g.history.HasSummarized(cacheItem.URL) {
			log.Printf("Skipping already summarized: %s", cacheItem.Title)
			continue
		}

		log.Printf("Summarizing: %s", cacheItem.Title)
		summary, err := g.llmClient.Summarize(cacheItem.Content)
		if err != nil {
			log.Printf("Error summarizing %s: %v", cacheItem.Title, err)
			continue
		}

		bankCategory := g.determineBankCategory(cacheItem.URL, cacheItem.Source)
		bankDir := filepath.Join(postsBaseDir, bankCategory)
		os.MkdirAll(bankDir, 0755)

		content := fmt.Sprintf(`---
title: "%s"
date: %s
source: "%s"
url: "%s"
category: "%s"
---

# [%s](%s)

%s
`, cacheItem.Title, timestamp, cacheItem.Source, cacheItem.URL, bankCategory, cacheItem.Title, cacheItem.URL, summary)

		filename := fmt.Sprintf("%s-%s.md", timestamp, sanitizeFilename(cacheItem.Title))
		if len(filename) > 100 {
			filename = filename[:100] + ".md"
		}

		path := filepath.Join(bankDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Printf("Error saving file %s: %v", path, err)
		} else {
			log.Printf("Saved report: %s", path)
			g.history.AddSummarized(cacheItem.URL)
		}
	}
	return nil
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "https://", "")
	s = strings.ReplaceAll(s, "http://", "")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "&", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func (g *Generator) determineBankCategory(link, source string) string {
	for domain, name := range g.bankMapping {
		if strings.Contains(link, domain) {
			return sanitizeFilename(name)
		}
	}
	// Fallback
	res := sanitizeFilename(source)
	if res == "" {
		return "Unknown"
	}
	return res
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return s
}

func (g *Generator) GenerateMonthly() error {
	outDir := filepath.Join("data", "posts")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}

	currentMonth := time.Now().Format("2006-01")
	var summariesBuilder strings.Builder

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Simple filter by filename date prefix YYYY-MM
		if strings.HasPrefix(entry.Name(), currentMonth) {
			content, err := os.ReadFile(filepath.Join(outDir, entry.Name()))
			if err != nil {
				log.Printf("Error reading %s: %v", entry.Name(), err)
				continue
			}
			summariesBuilder.Write(content)
			summariesBuilder.WriteString("\n---\n")
		}
	}

	aggregatedContent := summariesBuilder.String()
	if aggregatedContent == "" {
		log.Println("No reports found for this month.")
		return nil
	}

	log.Println("Generating Monthly Analysis...")
	analysis, err := g.llmClient.AnalyzeMonthly(aggregatedContent)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("Monthly Analysis - %s", currentMonth)
	filename := fmt.Sprintf("%s-Monthly_Analysis.md", currentMonth)

	finalContent := fmt.Sprintf(`---
title: "%s"
date: %s
type: "monthly"
---

# %s

%s
`, title, time.Now().Format("2006-01-02"), title, analysis)

	path := filepath.Join(outDir, filename)
	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		return err
	}

	log.Printf("Saved Monthly Analysis: %s", path)
	return nil
}
