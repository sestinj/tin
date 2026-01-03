package web

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

// WebServer serves the HTML web viewer for tin repositories
type WebServer struct {
	host     string
	port     int
	rootPath string
}

// NewWebServer creates a new web server instance
func NewWebServer(host string, port int, rootPath string) *WebServer {
	absPath, _ := filepath.Abs(rootPath)
	return &WebServer{
		host:     host,
		port:     port,
		rootPath: absPath,
	}
}

// Start starts the HTTP server
func (s *WebServer) Start() error {
	mux := http.NewServeMux()

	mux.Handle("/assets/", http.StripPrefix("/assets/", serveAssets()))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/repo/", s.handleRepo)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Printf("Tin web server listening on http://%s", addr)
	log.Printf("Serving repositories under: %s", s.rootPath)

	return http.ListenAndServe(addr, mux)
}
