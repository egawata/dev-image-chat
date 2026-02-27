package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"google.golang.org/genai"
)

// GeminiImageGenerator generates images using the Gemini API.
type GeminiImageGenerator struct {
	client    *genai.Client
	model     string
	outputDir string
	maxImages int
	mu        sync.Mutex
	generating bool
}

type GeminiImageGeneratorConfig struct {
	APIKey    string
	Model     string
	OutputDir string
}

func NewGeminiImageGenerator(cfg GeminiImageGeneratorConfig) (*GeminiImageGenerator, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &GeminiImageGenerator{
		client:    client,
		model:     cfg.Model,
		outputDir: cfg.OutputDir,
		maxImages: defaultMaxImages,
	}, nil
}

// Generate sends the prompt to Gemini and saves the resulting image.
// Returns the filename of the saved image. If generation is already in progress,
// it returns ("", nil) to indicate the request was skipped.
func (g *GeminiImageGenerator) Generate(prompt string) (string, error) {
	g.mu.Lock()
	if g.generating {
		g.mu.Unlock()
		log.Println("image generation already in progress, skipping")
		return "", nil
	}
	g.generating = true
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.generating = false
		g.mu.Unlock()
	}()

	ctx := context.Background()
	resp, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "3:4",
		},
	})
	if err != nil {
		return "", fmt.Errorf("Gemini image API error: %w", err)
	}

	imgData, err := extractImageFromResponse(resp)
	if err != nil {
		return "", err
	}

	filename, err := saveImage(g.outputDir, imgData)
	if err != nil {
		return "", err
	}

	cleanupOldImages(g.outputDir, g.maxImages)

	return filename, nil
}

// extractImageFromResponse extracts image bytes from a Gemini response.
func extractImageFromResponse(resp *genai.GenerateContentResponse) ([]byte, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}
	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("no content parts in Gemini response")
	}

	for _, part := range candidate.Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, nil
		}
	}

	return nil, fmt.Errorf("no image data found in Gemini response")
}
