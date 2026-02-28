package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
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

func (pg *OllamaPromptGenerator) Generate(ctx context.Context, messages []Message, sessionPath string) (string, error) {
	charIdx := pg.selectCharacterIndex(sessionPath)
	systemPrompt := pg.buildSystemPrompt(charIdx)

	if debugEnabled {
		if charIdx >= 0 {
			Debugf("using character index %d for session %q", charIdx, filepath.Base(sessionPath))
		}

		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			runes := []rune(lastMsg.Content)
			if len(runes) > 200 {
				runes = runes[:200]
			}
			preview := strconv.Quote(string(runes))
			Debugf("last message content (first 200 chars): %s", preview)
		}
	}

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
