package main

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "os"
 "strings"
 "time"
)

const groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"

type LLMClient struct {
 httpClient *http.Client
 apiKey string
 model string
}

func NewLLMClient() (*LLMClient, error) {
 key := os.Getenv("GROQ_API_KEY")
 if key == "" { return nil, fmt.Errorf("GROQ_API_KEY is required") }
 model := os.Getenv("GROQ_MODEL")
 if model == "" { model = "openai/gpt-oss-20b" }
 return &LLMClient{httpClient: &http.Client{Timeout: 45*time.Second}, apiKey: key, model: model}, nil
}

func (c *LLMClient) Generate(ctx context.Context, prompt string) (string, error) {
 payload := struct {
  Model string `json:"model"`
  Messages []map[string]string `json:"messages"`
 }{
  Model: c.model,
  Messages: []map[string]string{
   {"role": "system", "content": "You are ATLAS, a precise and helpful AI assistant."},
   {"role": "user", "content": prompt},
  },
 }
 body, err := json.Marshal(payload)
 if err != nil { return "", err }
 req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqEndpoint, bytes.NewReader(body))
 if err != nil { return "", err }
 req.Header.Set("Authorization", "Bearer " + c.apiKey)
 req.Header.Set("Content-Type", "application/json")
 resp, err := c.httpClient.Do(req)
 if err != nil { return "", err }
 defer resp.Body.Close()
 data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
 if err != nil { return "", err }
 var parsed struct {
  Choices []struct { Message struct { Content string `json:"content"` } `json:"message"` } `json:"choices"`
  Error struct { Message string `json:"message"` } `json:"error"`
 }
 if err := json.Unmarshal(data, &parsed); err != nil {
  return "", fmt.Errorf("Groq returned invalid JSON (HTTP %d): %w", resp.StatusCode, err)
 }
 if resp.StatusCode != http.StatusOK {
  return "", fmt.Errorf("Groq HTTP %d: %s", resp.StatusCode, parsed.Error.Message)
 }
 if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
  return "", fmt.Errorf("Groq returned an empty completion")
 }
 return parsed.Choices[0].Message.Content, nil
}
