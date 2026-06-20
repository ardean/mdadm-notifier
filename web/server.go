package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/ardean/mdadm-notifier/status"
)

//go:embed templates/index.html
var pageFS embed.FS

type Server struct {
	addr  string
	store *status.Store
	srv   *http.Server
}

func NewServer(addr string, store *status.Store) *Server {
	return &Server{
		addr:  addr,
		store: store,
	}
}

func (s *Server) Start() error {
	page, err := template.ParseFS(pageFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("parse dashboard template: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, nil); err != nil {
			log.Printf("web: failed to render dashboard: %v", err)
		}
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(s.store.Get()); err != nil {
			log.Printf("web: failed to encode status: %v", err)
		}
	})

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("web dashboard listening on http://%s", s.addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("web dashboard stopped: %v", err)
		}
	}()

	return nil
}

func (s *Server) Close() error {
	if s.srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
