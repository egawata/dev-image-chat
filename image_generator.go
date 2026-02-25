package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ImageGenerator struct {
	baseURL    string
	outputDir  string
	mu         sync.Mutex
	generating bool
}

type txt2imgRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt"`
	Steps          int     `json:"steps"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	CfgScale       float64 `json:"cfg_scale"`
	SamplerName    string  `json:"sampler_name"`
}

type txt2imgResponse struct {
	Images []string `json:"images"`
}

func NewImageGenerator(baseURL, outputDir string) (*ImageGenerator, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	return &ImageGenerator{
		baseURL:   baseURL,
		outputDir: outputDir,
	}, nil
}

// Generate sends the prompt to Stable Diffusion and saves the resulting image.
// Returns the filename of the saved image. If generation is already in progress,
// it returns ("", nil) to indicate the request was skipped.
func (ig *ImageGenerator) Generate(prompt string) (string, error) {
	ig.mu.Lock()
	if ig.generating {
		ig.mu.Unlock()
		log.Println("image generation already in progress, skipping")
		return "", nil
	}
	ig.generating = true
	ig.mu.Unlock()

	defer func() {
		ig.mu.Lock()
		ig.generating = false
		ig.mu.Unlock()
	}()

	fullPrompt := prompt + ", masterpiece, best quality, anime style, 1girl"
	negativePrompt := "lowres, bad anatomy, bad hands, text, error, ugly, duplicate, deformed, blurry, realistic, photo"

	reqBody := txt2imgRequest{
		Prompt:         fullPrompt,
		NegativePrompt: negativePrompt,
		Steps:          28,
		Width:          512,
		Height:         768,
		CfgScale:       5,
		SamplerName:    "Euler a",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := ig.baseURL + "/sdapi/v1/txt2img"
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("Stable Diffusion API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Stable Diffusion returned %d: %s", resp.StatusCode, string(body))
	}

	var result txt2imgResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Images) == 0 {
		return "", fmt.Errorf("no images in response")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Images[0])
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	filename := fmt.Sprintf("img_%d.png", time.Now().UnixMilli())
	filePath := filepath.Join(ig.outputDir, filename)

	if err := os.WriteFile(filePath, imgData, 0o644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	log.Printf("image saved: %s", filePath)
	return filename, nil
}
