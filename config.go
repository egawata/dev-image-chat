package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey       string
	GeminiModel        string
	SDBaseURL          string
	ServerPort         string
	ClaudeProjectDir   string
	DebounceInterval   time.Duration
	GenerateInterval   time.Duration
	RecentMessages     int
	CharacterSetting   string
	Debug              bool

	// Image generator selection: "sd" or "gemini"
	ImageGeneratorType string
	GeminiImageModel   string

	// Stable Diffusion image generation parameters
	SDSteps       int
	SDWidth       int
	SDHeight      int
	SDCfgScale    float64
	SDSamplerName string
	SDExtraPrompt string
}

func LoadConfig() (*Config, error) {
	// .env file is optional; environment variables take precedence
	_ = godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required")
	}

	sdBaseURL := os.Getenv("SD_BASE_URL")
	if sdBaseURL == "" {
		sdBaseURL = "http://localhost:7860"
	}

	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	claudeDir := os.Getenv("CLAUDE_PROJECTS_DIR")
	if claudeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		claudeDir = filepath.Join(home, ".claude", "projects")
	}

	characterSetting := ""
	characterFile := os.Getenv("CHARACTER_FILE")
	if characterFile != "" {
		data, err := os.ReadFile(characterFile)
		if err != nil {
			log.Printf("warning: could not read CHARACTER_FILE %q: %v", characterFile, err)
		} else {
			characterSetting = strings.TrimSpace(string(data))
		}
	}

	debug := os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"

	sdSteps := 28
	if v := os.Getenv("IMGCHAT_SD_STEPS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sdSteps = n
		} else {
			log.Printf("warning: invalid IMGCHAT_SD_STEPS %q, using default %d", v, sdSteps)
		}
	}

	sdWidth := 512
	if v := os.Getenv("IMGCHAT_SD_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sdWidth = n
		} else {
			log.Printf("warning: invalid IMGCHAT_SD_WIDTH %q, using default %d", v, sdWidth)
		}
	}

	sdHeight := 768
	if v := os.Getenv("IMGCHAT_SD_HEIGHT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sdHeight = n
		} else {
			log.Printf("warning: invalid IMGCHAT_SD_HEIGHT %q, using default %d", v, sdHeight)
		}
	}

	sdCfgScale := 5.0
	if v := os.Getenv("IMGCHAT_SD_CFG_SCALE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			sdCfgScale = f
		} else {
			log.Printf("warning: invalid IMGCHAT_SD_CFG_SCALE %q, using default %.1f", v, sdCfgScale)
		}
	}

	sdSamplerName := "Euler a"
	if v := os.Getenv("IMGCHAT_SD_SAMPLER_NAME"); v != "" {
		sdSamplerName = v
	}

	sdExtraPrompt := os.Getenv("IMGCHAT_SD_EXTRA_PROMPT")

	imageGeneratorType := strings.ToLower(os.Getenv("IMAGE_GENERATOR"))
	if imageGeneratorType == "" {
		imageGeneratorType = "sd"
	}
	if imageGeneratorType != "sd" && imageGeneratorType != "gemini" {
		return nil, fmt.Errorf("IMAGE_GENERATOR must be \"sd\" or \"gemini\", got %q", imageGeneratorType)
	}

	geminiImageModel := os.Getenv("GEMINI_IMAGE_MODEL")
	if geminiImageModel == "" {
		geminiImageModel = "gemini-2.5-flash-image"
	}

	generateInterval := 60 * time.Second
	if v := os.Getenv("GENERATE_INTERVAL"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			generateInterval = time.Duration(sec) * time.Second
		} else {
			log.Printf("warning: invalid GENERATE_INTERVAL %q, using default 60s", v)
		}
	}

	return &Config{
		GeminiAPIKey:       apiKey,
		GeminiModel:        geminiModel,
		SDBaseURL:          sdBaseURL,
		ServerPort:         serverPort,
		ClaudeProjectDir:   claudeDir,
		DebounceInterval:   3 * time.Second,
		GenerateInterval:   generateInterval,
		RecentMessages:     10,
		CharacterSetting:   characterSetting,
		Debug:              debug,
		ImageGeneratorType: imageGeneratorType,
		GeminiImageModel:   geminiImageModel,
		SDSteps:            sdSteps,
		SDWidth:            sdWidth,
		SDHeight:           sdHeight,
		SDCfgScale:         sdCfgScale,
		SDSamplerName:      sdSamplerName,
		SDExtraPrompt:      sdExtraPrompt,
	}, nil
}
