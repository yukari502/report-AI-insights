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
			Timeout: 60 * time.Second,
		},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func (c *Client) Summarize(content string) (string, error) {
	prompt := fmt.Sprintf(WeeklyPromptTemplate, content)
	return c.callLLM(prompt)
}

func (c *Client) AnalyzeMonthly(summaries string) (string, error) {
	prompt := fmt.Sprintf(MonthlyPromptTemplate, summaries)
	return c.callLLM(prompt)
}

func (c *Client) callLLM(prompt string) (string, error) {
	reqBody := chatRequest{
		Model: c.cfg.LLMModel,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.cfg.LLMApiURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.LLMApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.LLMApiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return chatResp.Choices[0].Message.Content, nil
}
