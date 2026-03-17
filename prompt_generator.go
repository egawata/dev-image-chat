package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/genai"
)

const baseSystemPrompt = `You are an image prompt generator for an anime-style illustration AI.
Given a conversation between a user and an AI assistant, generate a short English prompt
describing an anime-style illustration that captures the mood and situation of the latest
assistant message.

You MUST respond with a JSON object in the following format and nothing else:
{"prompt": "<your image prompt here>"}

Do NOT include any text outside the JSON object. No explanations, no markdown, no commentary.

Rules for the prompt value inside the JSON:
- The entire prompt MUST be in English only. Do NOT include any non-English characters, words, or text (no Japanese, Chinese, Korean, etc.). Even for in-scene text like signs, speech bubbles, or whiteboards, describe them in English or omit them.
- The prompt should describe a single anime girl character reacting to or representing
  the situation in the conversation.
- Include emotional expressions, poses, and background elements that match the context.
- Keep the prompt under 200 words.
- Do NOT include any negative prompts or technical parameters.`

// PromptGenerator is the interface for prompt generation backends.
type PromptGenerator interface {
	Generate(ctx context.Context, messages []Message, sessionPath string) (string, error)
}

// promptGeneratorBase contains shared logic for character selection and system prompt building.
type promptGeneratorBase struct {
	characterSettings []string
}

// selectCharacterIndex returns the character index for a given session path
// using FNV-1a hash of the session file basename.
// Returns -1 if no character settings are available.
func (b *promptGeneratorBase) selectCharacterIndex(sessionPath string) int {
	if len(b.characterSettings) == 0 {
		return -1
	}
	basename := filepath.Base(sessionPath)
	h := fnv.New32a()
	h.Write([]byte(basename))
	return int(h.Sum32() % uint32(len(b.characterSettings)))
}

// buildSystemPrompt constructs the full system prompt with character setting.
func (b *promptGeneratorBase) buildSystemPrompt(characterIndex int) string {
	sp := baseSystemPrompt
	if characterIndex >= 0 && characterIndex < len(b.characterSettings) {
		sp += "\n\nCharacter setting:\n" + b.characterSettings[characterIndex]
	}
	return sp
}

// max length of last message printed to log
const maxLastMsgLen = 200

// logDebugInfo logs character index and last message preview when debug mode is enabled.
func (b *promptGeneratorBase) logDebugInfo(sessionPath string, charIdx int, messages []Message) {
	if !debugEnabled {
		return
	}

	if charIdx >= 0 {
		Debugf("using character index %d for session %q", charIdx, filepath.Base(sessionPath))
	}

	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		runes := []rune(lastMsg.Content)
		if len(runes) > maxLastMsgLen {
			runes = runes[:maxLastMsgLen]
		}
		preview := strconv.Quote(string(runes))
		Debugf("last message content (first %d chars): %s", maxLastMsgLen, preview)
	}
}

// buildUserPrompt constructs the user prompt from messages.
func (b *promptGeneratorBase) buildUserPrompt(messages []Message) (string, error) {
	convJSON, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal messages: %w", err)
	}
	return fmt.Sprintf("Here is the recent conversation:\n%s\n\nGenerate an anime-style image prompt based on this conversation. Respond with ONLY a JSON object: {\"prompt\": \"<your prompt>\"}", string(convJSON)), nil
}

// promptResponse represents the expected JSON response from the LLM.
type promptResponse struct {
	Prompt string `json:"prompt"`
}

// jsonObjectPattern matches a JSON object containing a "prompt" key.
var jsonObjectPattern = regexp.MustCompile(`\{[^{}]*"prompt"\s*:\s*"[^"]*(?:\\.[^"]*)*"[^{}]*\}`)

// extractPromptFromResponse attempts to extract the image prompt from the LLM response.
// It first tries to parse the entire response as JSON. If that fails, it searches
// for a JSON object within the response text. If no JSON is found, it returns
// the original text as a fallback.
func extractPromptFromResponse(text string) string {
	text = strings.TrimSpace(text)

	// Try parsing the whole response as JSON
	var resp promptResponse
	if err := json.Unmarshal([]byte(text), &resp); err == nil && resp.Prompt != "" {
		return strings.TrimSpace(resp.Prompt)
	}

	// Try to find a JSON object embedded in the response
	if match := jsonObjectPattern.FindString(text); match != "" {
		if err := json.Unmarshal([]byte(match), &resp); err == nil && resp.Prompt != "" {
			return strings.TrimSpace(resp.Prompt)
		}
	}

	// Fallback: return original text (best effort)
	log.Printf("warning: LLM response was not valid JSON, using raw text")
	return text
}

// GeminiPromptGenerator generates prompts using the Gemini API.
type GeminiPromptGenerator struct {
	promptGeneratorBase
	client *genai.Client
	model  string
}

func NewGeminiPromptGenerator(apiKey, model string, characterSettings []string) (*GeminiPromptGenerator, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiPromptGenerator{
		promptGeneratorBase: promptGeneratorBase{
			characterSettings: characterSettings,
		},
		client: client,
		model:  model,
	}, nil
}

const (
	geminiTemperature     = 0.8
	geminiMaxOutputTokens = 8192
)

func (pg *GeminiPromptGenerator) Generate(ctx context.Context, messages []Message, sessionPath string) (string, error) {

	charIdx := pg.selectCharacterIndex(sessionPath)
	systemPrompt := pg.buildSystemPrompt(charIdx)
	pg.logDebugInfo(sessionPath, charIdx, messages)

	userPrompt, err := pg.buildUserPrompt(messages)
	if err != nil {
		return "", err
	}

	resp, err := pg.client.Models.GenerateContent(ctx, pg.model, genai.Text(userPrompt), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt, genai.RoleUser),
		Temperature:       genai.Ptr(float32(geminiTemperature)),
		MaxOutputTokens:   geminiMaxOutputTokens,
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

	return extractPromptFromResponse(text), nil
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
