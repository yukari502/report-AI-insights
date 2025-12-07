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
	cfg       *config.Config
	llmClient *llm.Client
}

func NewGenerator(cfg *config.Config) *Generator {
	return &Generator{
		cfg:       cfg,
		llmClient: llm.NewClient(cfg),
	}
}

func (g *Generator) GenerateWeekly() error {
	var wg sync.WaitGroup
	timestamp := time.Now().Format("2006-01-02")

	outDir := filepath.Join("data", "posts")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// 1. Build cache of processed URLs to avoid duplicates
	processedURLs := make(map[string]bool)
	entries, _ := os.ReadDir(outDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			content, _ := os.ReadFile(filepath.Join(outDir, entry.Name()))
			url := extractURLFromFrontmatter(string(content))
			if url != "" {
				processedURLs[url] = true
			}
		}
	}

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

			// Clean JSON string (sometimes LLM wraps in ```json ... ```)
			linksJSON = cleanJSON(linksJSON)

			var articleLinks []string
			if err := json.Unmarshal([]byte(linksJSON), &articleLinks); err != nil {
				log.Printf("Failed to parse links JSON for %s: %v\nJSON: %s", targetIndexURL, err, linksJSON)
				return
			}

			log.Printf("Found %d potential articles from %s", len(articleLinks), targetIndexURL)

			for _, link := range articleLinks {
				// Normalize link (handle relative paths if needed, though prompt asks for full URLs, we might need to fix them)
				if !strings.HasPrefix(link, "http") {
					// Extremely basic relative URL handling, might need improvement based on base URL
					// For now assume LLM does a good job or we skip
					continue
				}

				if processedURLs[link] {
					log.Printf("Skipping duplicate: %s", link)
					continue
				}

				// Step 3: Fetch Article Content
				log.Printf("Fetching: %s", link)
				article, err := crawler.FetchContent(link)
				if err != nil {
					log.Printf("Error crawling %s: %v", link, err)
					continue
				}

				// Step 4: Summarize
				log.Printf("Summarizing: %s", article.Title)
				summary, err := g.llmClient.Summarize(article.Content)
				if err != nil {
					log.Printf("Error summarizing %s: %v", link, err)
					continue
				}

				// Save to file
				content := fmt.Sprintf(`---
title: "%s"
date: %s
source: "%s"
url: "%s"
---

# %s

%s
`, article.Title, timestamp, article.Source, link, article.Title, summary)

				safeTitle := strings.ReplaceAll(article.Title, "/", "-")
				safeTitle = strings.ReplaceAll(safeTitle, " ", "_")
				filename := fmt.Sprintf("%s-%s.md", timestamp, safeTitle)
				if len(filename) > 100 {
					filename = filename[:100] + ".md"
				}

				path := filepath.Join(outDir, filename)
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

func extractURLFromFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "url:") {
			return strings.Trim(strings.TrimPrefix(line, "url:"), ` "`)
		}
	}
	return ""
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
