package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	imageDir := filepath.Join(".", "generated_images")

	promptGen, err := NewPromptGenerator(cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("prompt generator error: %v", err)
	}

	imageGen, err := NewImageGenerator(cfg.SDBaseURL, imageDir)
	if err != nil {
		log.Fatalf("image generator error: %v", err)
	}

	srv := NewServer(cfg.ServerPort, imageDir)

	watcher := NewWatcher(cfg.ClaudeProjectDir, cfg.DebounceInterval)

	// Channels for the pipeline
	promptCh := make(chan string, 4)
	imageCh := make(chan string, 4)

	done := make(chan struct{})

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup

	// File watcher goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := watcher.Run(done); err != nil {
			log.Printf("watcher error: %v", err)
		}
	}()

	// Conversation parser + prompt generation goroutine
	// Maintains per-file full message history for accurate context.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(promptCh)

		// Track full file content per path for re-parsing
		fileData := make(map[string][]byte)

		for {
			select {
			case <-done:
				return
			case ev, ok := <-watcher.Events():
				if !ok {
					return
				}

				// Append new data to the stored data for this file
				fileData[ev.Path] = append(fileData[ev.Path], ev.NewData...)

				// Parse the entire file's accumulated data
				messages := ParseJSONL(fileData[ev.Path])
				if len(messages) == 0 {
					continue
				}

				// Only generate when the last message is from the assistant
				last := messages[len(messages)-1]
				if last.Role != "assistant" {
					continue
				}

				recent := TailMessages(messages, cfg.RecentMessages)

				ctx := context.Background()
				prompt, err := promptGen.Generate(ctx, recent)
				if err != nil {
					log.Printf("prompt generation error: %v", err)
					continue
				}

				log.Printf("generated prompt: %s", prompt)

				select {
				case promptCh <- prompt:
				case <-done:
					return
				}
			}
		}
	}()

	// Image generation goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(imageCh)

		for {
			select {
			case <-done:
				return
			case prompt, ok := <-promptCh:
				if !ok {
					return
				}
				filename, err := imageGen.Generate(prompt)
				if err != nil {
					log.Printf("image generation error: %v", err)
					continue
				}
				if filename == "" {
					continue // skipped due to concurrent generation
				}

				select {
				case imageCh <- filename:
				case <-done:
					return
				}
			}
		}
	}()

	// Broadcast goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			case filename, ok := <-imageCh:
				if !ok {
					return
				}
				log.Printf("broadcasting new image: %s", filename)
				srv.Broadcast(filename)
			}
		}
	}()

	// HTTP server goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Start(); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	log.Printf("Claude Code Image Chat started")
	log.Printf("  Web UI: http://localhost:%s", cfg.ServerPort)
	log.Printf("  Watching: %s", cfg.ClaudeProjectDir)

	// Wait for shutdown signal
	<-sigCh
	log.Println("shutting down...")
	close(done)
	wg.Wait()
}
