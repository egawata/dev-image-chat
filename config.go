package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	RecentMessages     int
	CharacterSetting   string
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

	return &Config{
		GeminiAPIKey:       apiKey,
		GeminiModel:        geminiModel,
		SDBaseURL:          sdBaseURL,
		ServerPort:         serverPort,
		ClaudeProjectDir:   claudeDir,
		DebounceInterval:   3 * time.Second,
		RecentMessages:     10,
		CharacterSetting:   characterSetting,
	}, nil
}
