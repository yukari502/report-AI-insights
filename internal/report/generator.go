package report

import (
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

	for _, url := range g.cfg.TargetURLs {
		if url == "" {
			continue
		}
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()

			log.Printf("Fetching: %s", targetURL)
			article, err := crawler.FetchContent(targetURL)
			if err != nil {
				log.Printf("Error crawling %s: %v", targetURL, err)
				return
			}

			log.Printf("Summarizing: %s", article.Title)
			summary, err := g.llmClient.Summarize(article.Content)
			if err != nil {
				log.Printf("Error summarizing %s: %v", targetURL, err)
				return
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
`, article.Title, timestamp, article.Source, targetURL, article.Title, summary)

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
		}(url)
	}

	wg.Wait()
	return nil
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
