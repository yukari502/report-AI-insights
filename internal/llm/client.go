package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kaze/eco_report/internal/config"
)

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Gemini might take longer
		},
	}
}

// Google Gemini API Request/Response structures
type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *Client) Summarize(content string) (string, error) {
	prompt := fmt.Sprintf(WeeklyPromptTemplate, content)
	return c.callLLM(prompt)
}

func (c *Client) AnalyzeMonthly(summaries string) (string, error) {
	prompt := fmt.Sprintf(MonthlyPromptTemplate, summaries)
	return c.callLLM(prompt)
}

func (c *Client) ExtractLinks(htmlContent, currentWeek string) (string, error) {
	// Special method for AI Crawler
	prompt := fmt.Sprintf(`Analyze the following HTML content of a banking insights page.
Target Date Range: Current week (approx %s).
Task: Extract links to research articles or insights published within the target date range.
Ignore navigation links, footers, privacy policies, etc.
Ignore robots.txt.
Output Format: JSON array of strings, e.g. ["https://url1", "https://url2"].
If the article date is not explicitly visible but looks 'new' or 'featured', include it.

HTML Content:
%s`, currentWeek, htmlContent)
	return c.callLLM(prompt)
}

func (c *Client) callLLM(prompt string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// URL format: https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=API_KEY
	// We assume LLM_API_URL is the base, e.g., https://generativelanguage.googleapis.com/v1beta/models
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", c.cfg.LLMApiURL, c.cfg.LLMModel, c.cfg.LLMApiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var chatResp geminiResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Candidates) == 0 || len(chatResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return chatResp.Candidates[0].Content.Parts[0].Text, nil
}
