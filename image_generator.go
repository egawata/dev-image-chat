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
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultMaxImages = 30

// ImageGenerator is the interface for image generation backends.
type ImageGenerator interface {
	Generate(prompt string) (string, error)
}

// saveImage saves image data to the output directory with a timestamped filename.
// Returns the filename (not full path) of the saved image.
func saveImage(outputDir string, data []byte) (string, error) {
	filename := fmt.Sprintf("img_%d.png", time.Now().UnixMilli())
	filePath := filepath.Join(outputDir, filename)

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	log.Printf("image saved: %s", filePath)
	return filename, nil
}

// cleanupOldImages removes the oldest images when the number of images exceeds maxImages.
func cleanupOldImages(outputDir string, maxImages int) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		Debugf("cleanup: failed to read directory: %v", err)
		return
	}

	// Collect only regular .png files
	type fileWithTime struct {
		name    string
		modTime time.Time
	}
	var files []fileWithTime
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".png" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileWithTime{name: e.Name(), modTime: info.ModTime()})
	}

	if len(files) <= maxImages {
		return
	}

	// Sort by modification time ascending (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	toDelete := len(files) - maxImages
	for i := 0; i < toDelete; i++ {
		path := filepath.Join(outputDir, files[i].name)
		if err := os.Remove(path); err != nil {
			Debugf("cleanup: failed to remove %s: %v", path, err)
		} else {
			Debugf("cleanup: removed old image %s", files[i].name)
		}
	}
	Debugf("cleanup: removed %d old image(s), keeping %d", toDelete, maxImages)
}

// SDImageGenerator generates images using the Stable Diffusion WebUI API.
type SDImageGenerator struct {
	baseURL     string
	outputDir   string
	maxImages   int
	steps       int
	width       int
	height      int
	cfgScale    float64
	samplerName string
	extraPrompt    string
	extraNegPrompt string
	mu             sync.Mutex
	generating  bool
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

type SDImageGeneratorConfig struct {
	BaseURL     string
	OutputDir   string
	Steps       int
	Width       int
	Height      int
	CfgScale    float64
	SamplerName string
	ExtraPrompt    string
	ExtraNegPrompt string
}

func NewSDImageGenerator(igCfg SDImageGeneratorConfig) (*SDImageGenerator, error) {
	if err := os.MkdirAll(igCfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	return &SDImageGenerator{
		baseURL:     igCfg.BaseURL,
		outputDir:   igCfg.OutputDir,
		maxImages:   defaultMaxImages,
		steps:       igCfg.Steps,
		width:       igCfg.Width,
		height:      igCfg.Height,
		cfgScale:    igCfg.CfgScale,
		samplerName: igCfg.SamplerName,
		extraPrompt:    igCfg.ExtraPrompt,
		extraNegPrompt: igCfg.ExtraNegPrompt,
	}, nil
}

// Generate sends the prompt to Stable Diffusion and saves the resulting image.
// Returns the filename of the saved image. If generation is already in progress,
// it returns ("", nil) to indicate the request was skipped.
func (ig *SDImageGenerator) Generate(prompt string) (string, error) {
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

	fullPrompt := prompt
	if ig.extraPrompt != "" {
		trimmed := strings.TrimRight(fullPrompt, " ")
		if !strings.HasSuffix(trimmed, ",") {
			fullPrompt = trimmed + ", "
		}
		fullPrompt += ig.extraPrompt
	}
	negativePrompt := ig.extraNegPrompt

	reqBody := txt2imgRequest{
		Prompt:         fullPrompt,
		NegativePrompt: negativePrompt,
		Steps:          ig.steps,
		Width:          ig.width,
		Height:         ig.height,
		CfgScale:       ig.cfgScale,
		SamplerName:    ig.samplerName,
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

	filename, err := saveImage(ig.outputDir, imgData)
	if err != nil {
		return "", err
	}

	cleanupOldImages(ig.outputDir, ig.maxImages)

	return filename, nil
}
