package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const endpoint = "https://api.groq.com/openai/v1/chat/completions"

type Client struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

func NewClient() (*Client, error) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is required")
	}

	model := os.Getenv("GROQ_MODEL")
	if model == "" {
		model = "openai/gpt-oss-20b"
	}

	return &Client{
		httpClient: &http.Client{Timeout: 45 * time.Second},
		apiKey:     key,
		model:      model,
	}, nil
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are ATLAS, a precise and helpful AI assistant."},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq HTTP %d: %s", resp.StatusCode, result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("Groq returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
