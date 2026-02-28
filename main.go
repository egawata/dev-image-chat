package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	imageDir := filepath.Join(".", "generated_images")

	var promptGen PromptGenerator
	switch cfg.PromptGeneratorType {
	case "ollama":
		promptGen = NewOllamaPromptGenerator(cfg.OllamaBaseURL, cfg.OllamaModel, cfg.CharacterSettings)
	default:
		promptGen, err = NewGeminiPromptGenerator(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.CharacterSettings)
		if err != nil {
			log.Fatalf("prompt generator error: %v", err)
		}
	}

	var imageGen ImageGenerator
	switch cfg.ImageGeneratorType {
	case "gemini":
		imageGen, err = NewGeminiImageGenerator(GeminiImageGeneratorConfig{
			APIKey:    cfg.GeminiAPIKey,
			Model:     cfg.GeminiImageModel,
			OutputDir: imageDir,
		})
	default:
		imageGen, err = NewSDImageGenerator(SDImageGeneratorConfig{
			BaseURL:     cfg.SDBaseURL,
			OutputDir:   imageDir,
			Steps:       cfg.SDSteps,
			Width:       cfg.SDWidth,
			Height:      cfg.SDHeight,
			CfgScale:    cfg.SDCfgScale,
			SamplerName: cfg.SDSamplerName,
			ExtraPrompt:    cfg.SDExtraPrompt,
			ExtraNegPrompt: cfg.SDExtraNegPrompt,
		})
	}
	if err != nil {
		log.Fatalf("image generator error: %v", err)
	}

	InitLogger(cfg.Debug)

	done := make(chan struct{})

	srv := NewServer(cfg.ServerPort, imageDir, done)

	watcher := NewWatcher(cfg.ClaudeProjectDir, cfg.DebounceInterval)

	// Channels for the pipeline
	promptCh := make(chan PromptWithSession, 4)
	imageCh := make(chan SessionImage, 4)

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
	// Rate-limited: generates at most once per GenerateInterval, with a
	// trailing-edge timer so the final message in a burst is always processed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(promptCh)

		// Track full file content per path for re-parsing
		fileData := make(map[string][]byte)
		// Cache session titles so we only compute them once per session.
		// Capped to maxSessionTitles entries to prevent unbounded growth.
		const maxSessionTitles = 50
		sessionTitles := make(map[string]string)

		// Rate limiting state
		var lastGenTime time.Time
		var pendingRecent []Message
		var pendingPath string
		var deferredTimer *time.Timer
		timerCh := make(chan struct{}, 1)

		generatePrompt := func(recent []Message, sessionPath string) {
			ctx := context.Background()
			prompt, err := promptGen.Generate(ctx, recent, sessionPath)
			if err != nil {
				log.Printf("prompt generation error: %v", err)
				return
			}

			Debugf("generated prompt (%d chars): %q", len(prompt), prompt)

			sessionID := SessionIDFromPath(sessionPath)
			title, ok := sessionTitles[sessionID]
			if !ok {
				allMsgs := ParseJSONL(fileData[sessionPath])
				title = ExtractTitle(allMsgs, 30)
				if len(sessionTitles) >= maxSessionTitles {
					// Evict an arbitrary entry to keep the cache bounded
					for k := range sessionTitles {
						delete(sessionTitles, k)
						break
					}
				}
				sessionTitles[sessionID] = title
			}

			select {
			case promptCh <- PromptWithSession{
				Prompt:    prompt,
				SessionID: sessionID,
				Title:     title,
			}:
			case <-done:
			}
		}

		for {
			select {
			case <-done:
				if deferredTimer != nil {
					deferredTimer.Stop()
				}
				return

			case <-timerCh:
				// Deferred timer fired — generate with the latest pending data
				if pendingRecent != nil {
					if !srv.HasClients() {
						Debugf("no WebSocket clients connected, skipping deferred generation")
						pendingRecent = nil
						pendingPath = ""
						continue
					}
					Debugf("deferred generation triggered")
					lastGenTime = time.Now()
					recent := pendingRecent
					sessPath := pendingPath
					pendingRecent = nil
					pendingPath = ""
					generatePrompt(recent, sessPath)
				}

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

				// Skip generation when no WebSocket clients are connected
				if !srv.HasClients() {
					Debugf("no WebSocket clients connected, skipping image generation")
					continue
				}

				now := time.Now()
				if now.Sub(lastGenTime) >= cfg.GenerateInterval {
					// Enough time has passed — generate immediately
					if deferredTimer != nil {
						deferredTimer.Stop()
						deferredTimer = nil
					}
					pendingRecent = nil
					pendingPath = ""
					lastGenTime = now
					Debugf("immediate generation (%.0fs since last)", now.Sub(lastGenTime).Seconds())
					generatePrompt(recent, ev.Path)
				} else {
					// Too soon — defer to when the interval elapses
					pendingRecent = make([]Message, len(recent))
					copy(pendingRecent, recent)
					pendingPath = ev.Path
					if deferredTimer != nil {
						deferredTimer.Stop()
					}
					remaining := cfg.GenerateInterval - now.Sub(lastGenTime)
					Debugf("deferring generation (%.0fs remaining)", remaining.Seconds())
					deferredTimer = time.AfterFunc(remaining, func() {
						select {
						case timerCh <- struct{}{}:
						default:
						}
					})
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
			case ps, ok := <-promptCh:
				if !ok {
					return
				}
				filename, err := imageGen.Generate(ps.Prompt)
				if err != nil {
					log.Printf("image generation error: %v", err)
					continue
				}
				if filename == "" {
					continue // skipped due to concurrent generation
				}

				si := SessionImage{
					Filename:  filename,
					SessionID: ps.SessionID,
					Title:     ps.Title,
					UpdatedAt: time.Now().Format(time.RFC3339),
				}

				select {
				case imageCh <- si:
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
			case si, ok := <-imageCh:
				if !ok {
					return
				}
				Debugf("broadcasting new image: %s (session=%s)", si.Filename, si.SessionID)
				srv.BroadcastSessionImage(si)
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
	log.Printf("  Generate interval: %s", cfg.GenerateInterval)

	// Wait for shutdown signal
	<-sigCh
	log.Println("shutting down...")
	close(done)
	wg.Wait()
}
