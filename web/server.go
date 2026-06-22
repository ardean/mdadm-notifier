package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ardean/mdadm-notifier/status"
)

//go:embed templates/index.html static/*
var pageFS embed.FS

type RefreshFunc func()

type Server struct {
	addr        string
	mdDevice    string
	store       *status.Store
	onRefresh   RefreshFunc
	refreshLock sync.Mutex
	syncHub     *syncBroadcaster
	srv         *http.Server
}

func NewServer(addr string, mdDevice string, store *status.Store, onRefresh RefreshFunc) *Server {
	return &Server{
		addr:      addr,
		mdDevice:  mdDevice,
		store:     store,
		onRefresh: onRefresh,
		syncHub:   newSyncBroadcaster(store, mdDevice),
	}
}

func (s *Server) Start() error {
	page, err := template.ParseFS(pageFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("parse dashboard template: %w", err)
	}

	mux := http.NewServeMux()

	staticFS, err := fs.Sub(pageFS, "static")
	if err != nil {
		return fmt.Errorf("load static assets: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

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
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.writeStatus(w)
	})
	mux.HandleFunc("/api/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.writeSync(w)
	})
	mux.HandleFunc("/api/ws/sync", s.syncHub.handleWS)
	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.onRefresh == nil {
			http.Error(w, "refresh not configured", http.StatusServiceUnavailable)
			return
		}

		s.refreshLock.Lock()
		defer s.refreshLock.Unlock()
		s.onRefresh()
		s.writeStatus(w)
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

func (s *Server) writeStatus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(s.store.Get()); err != nil {
		log.Printf("web: failed to encode status: %v", err)
	}
}
