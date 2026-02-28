package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static/index.html
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	port     string
	imageDir string
	clients  map[*websocket.Conn]struct{}
	mu       sync.RWMutex
	done     <-chan struct{}
}

func NewServer(port, imageDir string, done <-chan struct{}) *Server {
	return &Server{
		port:     port,
		imageDir: imageDir,
		clients:  make(map[*websocket.Conn]struct{}),
		done:     done,
	}
}

// HasClients returns true if at least one WebSocket client is connected.
func (s *Server) HasClients() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients) > 0
}

// BroadcastSessionImage sends a SessionImage as JSON to all connected WebSocket clients.
func (s *Server) BroadcastSessionImage(si SessionImage) {
	data, err := json.Marshal(si)
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("websocket write error: %v", err)
		}
	}
}

// Start begins serving HTTP and WebSocket connections. It blocks until
// the done channel is closed, then gracefully shuts down the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Serve index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// Serve generated images
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir(s.imageDir))))

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWS)

	httpServer := &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
	}

	// Shut down the HTTP server when done is closed.
	go func() {
		<-s.done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("http server shutdown error: %v", err)
		}
	}()

	log.Printf("server listening on :%s", s.port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	log.Printf("WebSocket client connected (total: %d)", len(s.clients))

	// Keep connection alive; remove on close.
	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected (total: %d)", len(s.clients))
	}()

	// Close the connection when done is signaled so ReadMessage unblocks.
	go func() {
		<-s.done
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
