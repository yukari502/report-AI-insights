package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/go-shiori/go-readability"
)

type Article struct {
	URL     string
	Title   string
	Content string
	Source  string
}

// FetchRawHTML uses chromedp to fetch and render the page (executing JS)
func FetchRawHTML(urlStr string) (string, error) {
	// 1. Create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// 2. Timeout context
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var htmlContent string

	// 3. Run tasks
	// We wait for body to be visible to ensure some render happened.
	// We also sleep a bit to allow XHRs to complete for SPAs.
	err := chromedp.Run(ctx,
		chromedp.Navigate(urlStr),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // Wait for dynamic content
		chromedp.OuterHTML("html", &htmlContent),
	)

	if err != nil {
		return "", fmt.Errorf("chromedp error: %w", err)
	}

	return htmlContent, nil
}

func FetchContent(urlString string) (*Article, error) {
	// 1. Fetch raw first (rendered)
	rawHTML, err := FetchRawHTML(urlString)
	if err != nil {
		return nil, err
	}

	// 2. Parse content from the rendered HTML using readability
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	doc, err := readability.FromReader(strings.NewReader(rawHTML), parsedURL)
	if err != nil {
		return nil, err
	}

	return &Article{
		URL:     urlString,
		Title:   doc.Title,
		Content: doc.TextContent,
		Source:  doc.SiteName,
	}, nil
}
