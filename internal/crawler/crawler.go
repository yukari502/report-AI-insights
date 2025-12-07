package crawler

import (
	"fmt"
	"time"

	"github.com/go-shiori/go-readability"
)

type Article struct {
	URL     string
	Title   string
	Content string
	Source  string
}

func FetchContent(url string) (*Article, error) {
	// 5s timeout
	article, err := readability.FromURL(url, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}

	return &Article{
		URL:     url,
		Title:   article.Title,
		Content: article.TextContent,
		Source:  article.SiteName,
	}, nil
}
