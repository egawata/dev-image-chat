package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaPromptGenerator generates prompts using a local ollama instance.
type OllamaPromptGenerator struct {
	promptGeneratorBase
	baseURL     string
	model       string
	temperature float64
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  ollamaChatOptions   `json:"options"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatOptions struct {
	Temperature float64 `json:"temperature"`
}

type ollamaChatResponse struct {
	Message ollamaChatMessage `json:"message"`
}

func NewOllamaPromptGenerator(baseURL, model string, characterSettings []string) *OllamaPromptGenerator {
	return &OllamaPromptGenerator{
		promptGeneratorBase: promptGeneratorBase{
			characterSettings: characterSettings,
		},
		baseURL:     baseURL,
		model:       model,
		temperature: 0.8,
	}
}

// CheckConnection verifies that the Ollama server is reachable and the
// configured model is available. It returns nil on success, or an error
// describing what went wrong.
func (pg *OllamaPromptGenerator) CheckConnection(ctx context.Context) error {
	url := strings.TrimRight(pg.baseURL, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to Ollama at %s: %w", pg.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	// Check if the configured model is available
	for _, m := range result.Models {
		// Model names may include a tag (e.g. "gemma3:latest"), so match
		// both exact name and name without tag.
		name := strings.Split(m.Name, ":")[0]
		if m.Name == pg.model || name == pg.model {
			return nil
		}
	}

	available := make([]string, len(result.Models))
	for i, m := range result.Models {
		available[i] = m.Name
	}
	return fmt.Errorf("model %q not found in Ollama (available: %s)", pg.model, strings.Join(available, ", "))
}

func (pg *OllamaPromptGenerator) Generate(ctx context.Context, messages []Message, sessionPath string) (string, error) {
	charIdx := pg.selectCharacterIndex(sessionPath)
	systemPrompt := pg.buildSystemPrompt(charIdx)
	pg.logDebugInfo(sessionPath, charIdx, messages)

	userPrompt, err := pg.buildUserPrompt(messages)
	if err != nil {
		return "", err
	}

	reqBody := ollamaChatRequest{
		Model: pg.model,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream:  false,
		Options: ollamaChatOptions{Temperature: pg.temperature},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	url := strings.TrimRight(pg.baseURL, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(body))
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %w", err)
	}

	text := strings.TrimSpace(result.Message.Content)
	if text == "" {
		return "", fmt.Errorf("empty response from ollama")
	}

	return text, nil
}
