package main

import (
	"embed"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

//go:embed static/index.html
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	port      string
	imageDir  string
	clients   map[*websocket.Conn]struct{}
	mu        sync.RWMutex
}

func NewServer(port, imageDir string) *Server {
	return &Server{
		port:     port,
		imageDir: imageDir,
		clients:  make(map[*websocket.Conn]struct{}),
	}
}

// Broadcast sends the image filename to all connected WebSocket clients.
func (s *Server) Broadcast(filename string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for conn := range s.clients {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(filename)); err != nil {
			log.Printf("websocket write error: %v", err)
		}
	}
}

// Start begins serving HTTP and WebSocket connections. It blocks.
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

	log.Printf("server listening on :%s", s.port)
	return http.ListenAndServe(":"+s.port, mux)
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

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
