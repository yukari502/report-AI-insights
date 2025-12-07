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
	"github.com/kaze/eco_report/internal/llm"
)

type Generator struct {
	cfg         *config.Config
	llmClient   *llm.Client
	bankMapping map[string]string
}

func NewGenerator(cfg *config.Config) *Generator {
	g := &Generator{
		cfg:       cfg,
		llmClient: llm.NewClient(cfg),
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

func (g *Generator) GenerateWeekly() error {
	var wg sync.WaitGroup
	timestamp := time.Now().Format("2006-01-02")

	// Cache Directory: data/cache/YYYY-MM-DD
	cacheDir := filepath.Join("data", "cache", timestamp)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	// Posts Base Directory
	postsBaseDir := filepath.Join("data", "posts")
	if err := os.MkdirAll(postsBaseDir, 0755); err != nil {
		return err
	}

	// 1. Build dedup map from existing posts
	processedURLs := g.loadProcessedURLs(postsBaseDir)

	for _, indexURL := range g.cfg.TargetURLs {
		if indexURL == "" {
			continue
		}
		wg.Add(1)
		go func(targetIndexURL string) {
			defer wg.Done()

			log.Printf("AI Crawling Index: %s", targetIndexURL)

			// Step 1: Fetch Index Page
			rawHTML, err := crawler.FetchRawHTML(targetIndexURL)
			if err != nil {
				log.Printf("Failed to fetch index %s: %v", targetIndexURL, err)
				return
			}

			// Step 2: Use LLM to discover article links
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

			log.Printf("Found %d articles from %s", len(articleLinks), targetIndexURL)

			for _, link := range articleLinks {
				if !strings.HasPrefix(link, "http") {
					continue
				}

				if processedURLs[link] {
					log.Printf("Skipping duplicate (already posted): %s", link)
					continue
				}

				// Generate Cache Filename (Base64 of URL to be safe)
				// Actually, simple hash or sanitized string is fine
				safeName := sanitizeFilename(link)
				cacheFile := filepath.Join(cacheDir, safeName+".json")

				var cacheItem CacheItem

				// Check if already in today's cache
				if data, err := os.ReadFile(cacheFile); err == nil {
					log.Printf("Hit Cache: %s", link)
					json.Unmarshal(data, &cacheItem)
				} else {
					// Cache Miss: Fetch
					log.Printf("Fetching & Caching: %s", link)
					article, err := crawler.FetchContent(link)
					if err != nil {
						log.Printf("Error crawling %s: %v", link, err)
						continue
					}

					cacheItem = CacheItem{
						URL:         link,
						Title:       article.Title,
						Content:     article.Content,
						Source:      article.Source,
						FetchedDate: timestamp,
					}

					// Save to Cache
					data, _ := json.MarshalIndent(cacheItem, "", "  ")
					os.WriteFile(cacheFile, data, 0644)
				}

				// Step 4: Summarize (Analyze)
				log.Printf("Summarizing: %s", cacheItem.Title)
				summary, err := g.llmClient.Summarize(cacheItem.Content)
				if err != nil {
					log.Printf("Error summarizing %s: %v", link, err)
					continue
				}

				// Determine Bank Category (Directory)
				// Load mapping (lazy load or load once, here lazy load for simplicity else pass to struct)
				// For efficiency, let's just do it here or better, load it in NewGenerator?
				// To keep change localized, I'll load it here once or reused.
				// Since this runs in a goroutine, parallel read is fine if map is read-only.
				// Let's implement a helper `determineBankCategory` that reads the file once effectively.

				bankCategory := g.determineBankCategory(link, cacheItem.Source)

				bankDir := filepath.Join(postsBaseDir, bankCategory)
				if err := os.MkdirAll(bankDir, 0755); err != nil {
					log.Printf("Failed to create bank dir %s: %v", bankDir, err)
					continue
				}

				// Save Report
				content := fmt.Sprintf(`---
title: "%s"
date: %s
source: "%s"
url: "%s"
category: "%s"
---

# [%s](%s)

%s
`, cacheItem.Title, timestamp, cacheItem.Source, link, bankCategory, cacheItem.Title, link, summary)

				filename := fmt.Sprintf("%s-%s.md", timestamp, sanitizeFilename(cacheItem.Title))
				if len(filename) > 100 {
					filename = filename[:100] + ".md"
				}

				path := filepath.Join(bankDir, filename)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					log.Printf("Error saving file %s: %v", path, err)
				} else {
					log.Printf("Saved report: %s", path)
				}
			}
		}(indexURL)
	}

	wg.Wait()
	return nil
}

func (g *Generator) loadProcessedURLs(rootDir string) map[string]bool {
	processed := make(map[string]bool)
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, _ := os.ReadFile(path)
			url := extractURLFromFrontmatter(string(content))
			if url != "" {
				processed[url] = true
			}
		}
		return nil
	})
	return processed
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

func extractURLFromFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "url:") {
			return strings.Trim(strings.TrimPrefix(line, "url:"), ` "`)
		}
	}
	return ""
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
