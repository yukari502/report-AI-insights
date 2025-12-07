package main

import (
	"flag"
	"log"

	"github.com/kaze/eco_report/internal/config"
	"github.com/kaze/eco_report/internal/report"
	"github.com/kaze/eco_report/internal/site"
)

func main() {
	mode := flag.String("mode", "weekly", "Mode to run: weekly, monthly")
	step := flag.String("step", "all", "Step to run (for weekly): all, crawl, summarize, site")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting ECO Report in %s mode (step: %s)", *mode, *step)
	log.Printf("Loaded configuration with %d target URLs", len(cfg.TargetURLs))

	switch *mode {
	case "weekly":
		runWeekly(cfg, *step)
	case "monthly":
		runMonthly(cfg)
	default:
		log.Printf("Unknown mode: %s. Use 'weekly' or 'monthly'", *mode)
	}
}

func runWeekly(cfg *config.Config, step string) {
	gen := report.NewGenerator(cfg)

	switch step {
	case "crawl":
		log.Println("Step: Crawling...")
		if err := gen.FetchAll(); err != nil {
			log.Fatalf("Crawl failed: %v", err)
		}
	case "summarize":
		log.Println("Step: Summarizing...")
		if err := gen.SummarizeAll(); err != nil {
			log.Fatalf("Summarize failed: %v", err)
		}
	case "site":
		log.Println("Step: Generating Site...")
		if err := site.GenerateSite("data/posts", "."); err != nil {
			log.Fatalf("Site generation failed: %v", err)
		}
	case "all":
		log.Println("Running Full Weekly Workflow...")
		if err := gen.FetchAll(); err != nil {
			log.Fatalf("Crawl failed: %v", err)
		}
		log.Println("Crawling complete. Starting summarization...")
		if err := gen.SummarizeAll(); err != nil {
			log.Fatalf("Summarize failed: %v", err)
		}
		log.Println("Generating Static Site...")
		if err := site.GenerateSite("data/posts", "."); err != nil {
			log.Fatalf("Site generation failed: %v", err)
		}
	default:
		log.Fatalf("Unknown step: %s", step)
	}
}

func runMonthly(cfg *config.Config) {
	log.Println("Running Monthly Report generation...")
	gen := report.NewGenerator(cfg)
	if err := gen.GenerateMonthly(); err != nil {
		log.Fatalf("Monthly generation failed: %v", err)
	}

	// Generate Site
	log.Println("Generating Static Site...")
	if err := site.GenerateSite("data/posts", "."); err != nil {
		log.Fatalf("Site generation failed: %v", err)
	}
}
