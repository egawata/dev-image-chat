package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

const baseSystemPrompt = `You are an image prompt generator for an anime-style illustration AI.
Given a conversation between a user and an AI assistant, generate a short English prompt
describing an anime-style illustration that captures the mood and situation of the latest
assistant message.

Rules:
- Output ONLY the image prompt, nothing else.
- The prompt should describe a single anime girl character reacting to or representing
  the situation in the conversation.
- Include emotional expressions, poses, and background elements that match the context.
- Keep the prompt under 200 words.
- Do NOT include any negative prompts or technical parameters.`

type PromptGenerator struct {
	client       *genai.Client
	model        string
	systemPrompt string
}

func NewPromptGenerator(apiKey, model, characterSetting string) (*PromptGenerator, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	sp := baseSystemPrompt
	if characterSetting != "" {
		sp += "\n\nCharacter setting:\n" + characterSetting
	}

	return &PromptGenerator{
		client:       client,
		model:        model,
		systemPrompt: sp,
	}, nil
}

func (pg *PromptGenerator) Generate(ctx context.Context, messages []Message) (string, error) {
	convJSON, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages: %w", err)
	}

	userPrompt := fmt.Sprintf("Here is the recent conversation:\n%s\n\nGenerate an anime-style image prompt based on this conversation.", string(convJSON))

	resp, err := pg.client.Models.GenerateContent(ctx, pg.model, genai.Text(userPrompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(pg.systemPrompt, genai.RoleUser),
		Temperature:       genai.Ptr(float32(0.8)),
		MaxOutputTokens:   300,
	})
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
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
