package crawler

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

type Article struct {
	URL     string
	Title   string
	Content string
	Source  string
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func FetchRawHTML(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	// Mimic browser to avoid some bot blocks, and ignores robots.txt by nature of being a custom client
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func FetchContent(urlString string) (*Article, error) {
	// 1. Fetch raw first to ensure we can access it
	// Actually go-readability handles fetching too, but let's use our client configuration
	// go-readability FromURL doesn't easily allow custom headers unless we use FromReader

	rawHTML, err := FetchRawHTML(urlString)
	if err != nil {
		return nil, err
	}

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
