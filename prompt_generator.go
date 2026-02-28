package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/genai"
)

const baseSystemPrompt = `You are an image prompt generator for an anime-style illustration AI.
Given a conversation between a user and an AI assistant, generate a short English prompt
describing an anime-style illustration that captures the mood and situation of the latest
assistant message.

Rules:
- Output ONLY the image prompt, nothing else.
- The entire output MUST be in English only. Do NOT include any non-English characters, words, or text (no Japanese, Chinese, Korean, etc.). Even for in-scene text like signs, speech bubbles, or whiteboards, describe them in English or omit them.
- The prompt should describe a single anime girl character reacting to or representing
  the situation in the conversation.
- Include emotional expressions, poses, and background elements that match the context.
- Keep the prompt under 200 words.
- Do NOT include any negative prompts or technical parameters.`

type PromptGenerator struct {
	client            *genai.Client
	model             string
	baseSystemPrompt  string
	characterSettings []string
}

func NewPromptGenerator(apiKey, model string, characterSettings []string) (*PromptGenerator, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &PromptGenerator{
		client:            client,
		model:             model,
		baseSystemPrompt:  baseSystemPrompt,
		characterSettings: characterSettings,
	}, nil
}

// selectCharacterIndex returns the character index for a given session path
// using FNV-1a hash of the session file basename.
// Returns -1 if no character settings are available.
func (pg *PromptGenerator) selectCharacterIndex(sessionPath string) int {
	if len(pg.characterSettings) == 0 {
		return -1
	}
	basename := filepath.Base(sessionPath)
	h := fnv.New32a()
	h.Write([]byte(basename))
	return int(h.Sum32() % uint32(len(pg.characterSettings)))
}

// buildSystemPrompt constructs the full system prompt with character setting.
func (pg *PromptGenerator) buildSystemPrompt(characterIndex int) string {
	sp := pg.baseSystemPrompt
	if characterIndex >= 0 && characterIndex < len(pg.characterSettings) {
		sp += "\n\nCharacter setting:\n" + pg.characterSettings[characterIndex]
	}
	return sp
}

func (pg *PromptGenerator) Generate(ctx context.Context, messages []Message, sessionPath string) (string, error) {

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

	convJSON, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages: %w", err)
	}

	userPrompt := fmt.Sprintf("Here is the recent conversation:\n%s\n\nGenerate an anime-style image prompt based on this conversation.", string(convJSON))

	resp, err := pg.client.Models.GenerateContent(ctx, pg.model, genai.Text(userPrompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		Temperature:       genai.Ptr(float32(0.8)),
		MaxOutputTokens:   8192,
	})
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	if resp != nil && len(resp.Candidates) > 0 {
		reason := resp.Candidates[0].FinishReason
		if reason != genai.FinishReasonStop {
			log.Printf("warning: Gemini finish reason: %s", reason)
		}
	}

	text := extractTextFromResponse(resp)
	if text == "" {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return strings.TrimSpace(text), nil
}

func extractTextFromResponse(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return ""
	}
	var parts []string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, "")
}
