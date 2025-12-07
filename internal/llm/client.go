package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/yukari502/report-AI-insights/internal/config"
)

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

// --- Google Gemini Structs ---
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

// --- OpenAI / DeepSeek Structs ---
type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// --- Methods ---

func (c *Client) Summarize(content string) (string, error) {
	prompt := fmt.Sprintf(WeeklyPromptTemplate, c.cfg.OutputLanguage, content)
	// Analyzer uses its own config
	return c.dispatch(prompt, c.cfg.LLMAnalyzerApiURL, c.cfg.LLMAnalyzerModel, c.cfg.LLMAnalyzerApiKey)
}

func (c *Client) AnalyzeMonthly(summaries string) (string, error) {
	prompt := fmt.Sprintf(MonthlyPromptTemplate, c.cfg.OutputLanguage, summaries)
	// Analyzer uses its own config
	return c.dispatch(prompt, c.cfg.LLMAnalyzerApiURL, c.cfg.LLMAnalyzerModel, c.cfg.LLMAnalyzerApiKey)
}

func (c *Client) ExtractLinks(htmlContent, currentWeek string) (string, error) {
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

	// Crawler uses its own config
	return c.dispatch(prompt, c.cfg.LLMCrawlerApiURL, c.cfg.LLMCrawlerModel, c.cfg.LLMCrawlerApiKey)
}

// dispatch routes the request to proper provider handler based on URL or logic
func (c *Client) dispatch(prompt, apiURL, model, apiKey string) (string, error) {
	// Simple heuristic: if URL contains "googleapis", it's Gemini
	if strings.Contains(apiURL, "googleapis") {
		return c.callGemini(prompt, apiURL, model, apiKey)
	}
	// Default to OpenAI compatible (DeepSeek, etc.)
	return c.callOpenAI(prompt, apiURL, model, apiKey)
}

func (c *Client) callGemini(prompt, apiURL, model, apiKey string) (string, error) {
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

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", apiURL, model, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.doRequest(req, func(body []byte) (string, error) {
		var chatResp geminiResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return "", err
		}
		if len(chatResp.Candidates) == 0 || len(chatResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no response from Gemini")
		}
		return chatResp.Candidates[0].Content.Parts[0].Text, nil
	})
}

func (c *Client) callOpenAI(prompt, apiURL, model, apiKey string) (string, error) {
	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// OpenAI/DeepSeek usually expects /chat/completions appended if base url is provided
	// But let's assume the user provides the base URL e.g., https://api.deepseek.com/chat/completions
	// Or we can be smart. DeepSeek doc says: https://api.deepseek.com/chat/completions
	// If the user configures LLM_CRAWLER_API_URL=https://api.deepseek.com/chat/completions

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	return c.doRequest(req, func(body []byte) (string, error) {
		var chatResp openAIResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return "", err
		}
		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("no response from OpenAI/DeepSeek: %s", string(body))
		}
		content := chatResp.Choices[0].Message.Content

		// Remove <think>...</think> blocks (common in DeepSeek R1/V3)
		re := regexp.MustCompile(`(?s)<think>.*?</think>`)
		content = re.ReplaceAllString(content, "")

		return strings.TrimSpace(content), nil
	})
}

func (c *Client) doRequest(req *http.Request, parser func([]byte) (string, error)) (string, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return parser(body)
}
