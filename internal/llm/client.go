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
	// Inject language into prompt
	prompt := fmt.Sprintf(WeeklyPromptTemplate, c.cfg.OutputLanguage, content)
	return c.callLLM(prompt, c.cfg.LLMAnalyzerApiURL, c.cfg.LLMAnalyzerModel)
}

func (c *Client) AnalyzeMonthly(summaries string) (string, error) {
	prompt := fmt.Sprintf(MonthlyPromptTemplate, c.cfg.OutputLanguage, summaries)
	return c.callLLM(prompt, c.cfg.LLMAnalyzerApiURL, c.cfg.LLMAnalyzerModel)
}

func (c *Client) ExtractLinks(htmlContent, currentWeek string) (string, error) {
	// Special method for AI Crawler
	// We want articles from the LAST MONTH (as per user request "not older than 1 month")?
	// User said: "6. 不会抓取过时的文章（距今一个月以上）"
	// But "1. 这一部分可以按照日期分类...仅读取最新的部分"
	// Let's ask LLM to discover articles from "Recent Month" to be safe, then we filter strictly later.

	prompt := fmt.Sprintf(`Analyze the following HTML content of a banking insights page.
Target: Find research articles or insights published within the LAST MONTH.
Current Date: %s.
Task: Extract links to these articles.
Ignore navigation links, footers, privacy policies, etc.
Ignore robots.txt.
Output Format: JSON array of strings, e.g. ["https://url1", "https://url2"].
If the article date is not explicitly visible but looks 'new' or 'featured', include it.

HTML Content:
%s`, time.Now().Format("2006-01-02"), htmlContent)
	return c.callLLM(prompt, c.cfg.LLMCrawlerApiURL, c.cfg.LLMCrawlerModel)
}

func (c *Client) callLLM(prompt, apiURL, model string) (string, error) {
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

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", apiURL, model, c.cfg.LLMApiKey)

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
