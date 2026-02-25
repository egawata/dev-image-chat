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
	}, nil
}
