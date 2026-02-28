package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	CharactersDir      string
	CharacterSettings  []string
	Debug              bool

	// Prompt generator selection: "gemini" or "ollama"
	PromptGeneratorType string
	OllamaBaseURL       string
	OllamaModel         string

	// Image generator selection: "sd" or "gemini"
	ImageGeneratorType string
	GeminiImageModel   string

	// Stable Diffusion image generation parameters
	SDSteps       int
	SDWidth       int
	SDHeight      int
	SDCfgScale    float64
	SDSamplerName string
	SDExtraPrompt    string
	SDExtraNegPrompt string
}

func LoadConfig() (*Config, error) {
	// .env file is optional; environment variables take precedence
	_ = godotenv.Load()

	promptGeneratorType := strings.ToLower(os.Getenv("PROMPT_GENERATOR"))
	if promptGeneratorType == "" {
		promptGeneratorType = "gemini"
	}
	if promptGeneratorType != "gemini" && promptGeneratorType != "ollama" {
		return nil, fmt.Errorf("PROMPT_GENERATOR must be \"gemini\" or \"ollama\", got %q", promptGeneratorType)
	}

	ollamaBaseURL := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseURL == "" {
		ollamaBaseURL = "http://localhost:11434"
	}

	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = "gemma3"
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

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

	charactersDir := os.Getenv("CHARACTERS_DIR")
	if charactersDir == "" {
		charactersDir = "characters"
	}

	characterSettings, err := loadCharacterSettings(charactersDir)
	if err != nil {
		log.Printf("warning: could not load characters from %q: %v", charactersDir, err)
	}

	// Fallback to CHARACTER_FILE if no characters found in directory
	if len(characterSettings) == 0 {
		characterFile := os.Getenv("CHARACTER_FILE")
		if characterFile != "" {
			data, err := os.ReadFile(characterFile)
			if err != nil {
				log.Printf("warning: could not read CHARACTER_FILE %q: %v", characterFile, err)
			} else {
				setting := strings.TrimSpace(string(data))
				if setting != "" {
					characterSettings = []string{setting}
				}
			}
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
	sdExtraNegPrompt := os.Getenv("IMGCHAT_SD_EXTRA_NEG_PROMPT")

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

	// GEMINI_API_KEY is required when prompt generator or image generator uses Gemini
	needsGeminiKey := promptGeneratorType == "gemini" || imageGeneratorType == "gemini"
	if needsGeminiKey && apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is required when prompt generator or image generator is \"gemini\"")
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
		GeminiAPIKey:        apiKey,
		GeminiModel:         geminiModel,
		SDBaseURL:           sdBaseURL,
		PromptGeneratorType: promptGeneratorType,
		OllamaBaseURL:       ollamaBaseURL,
		OllamaModel:         ollamaModel,
		ServerPort:         serverPort,
		ClaudeProjectDir:   claudeDir,
		DebounceInterval:   3 * time.Second,
		GenerateInterval:   generateInterval,
		RecentMessages:     10,
		CharactersDir:      charactersDir,
		CharacterSettings:  characterSettings,
		Debug:              debug,
		ImageGeneratorType: imageGeneratorType,
		GeminiImageModel:   geminiImageModel,
		SDSteps:            sdSteps,
		SDWidth:            sdWidth,
		SDHeight:           sdHeight,
		SDCfgScale:         sdCfgScale,
		SDSamplerName:      sdSamplerName,
		SDExtraPrompt:      sdExtraPrompt,
		SDExtraNegPrompt:   sdExtraNegPrompt,
	}, nil
}

// loadCharacterSettings reads all .md files from the specified directory,
// sorted by filename, and returns their contents.
func loadCharacterSettings(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var settings []string
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			log.Printf("warning: could not read character file %q: %v", name, err)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			settings = append(settings, content)
			log.Printf("loaded character setting: %s", name)
		}
	}
	return settings, nil
}
