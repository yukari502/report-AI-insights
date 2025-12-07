package main

import (
	"flag"
	"log"

	"github.com/kaze/eco_report/internal/config"
	"github.com/kaze/eco_report/internal/report"
	"github.com/kaze/eco_report/internal/site"
)

func main() {
	mode := flag.String("mode", "weekly", "Mode to run: weekly, monthly, or test")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting ECO Report in %s mode", *mode)
	log.Printf("Loaded configuration with %d target URLs", len(cfg.TargetURLs))

	switch *mode {
	case "weekly":
		runWeekly(cfg)
	case "monthly":
		runMonthly(cfg)
	default:
		log.Printf("Unknown mode: %s. Use 'weekly' or 'monthly'", *mode)
	}
}

func runWeekly(cfg *config.Config) {
	log.Println("Running Weekly Report generation...")
	gen := report.NewGenerator(cfg)
	// Run Generator
	if err := gen.GenerateWeekly(); err != nil {
		log.Fatalf("Weekly generation failed: %v", err)
	}

	// Generate Site
	log.Println("Generating Static Site...")
	if err := site.GenerateSite("data/posts", "."); err != nil {
		log.Fatalf("Site generation failed: %v", err)
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
