package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
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

type SelfTestFunc func(device, testType string) error

type SelfTestStatusFunc func(device string) (SelfTestPoll, error)

type StatusError struct {
	Code int
	Msg  string
}

func (e StatusError) Error() string {
	return e.Msg
}

type SelfTestPoll struct {
	Device      string             `json:"device"`
	InProgress  bool               `json:"in_progress"`
	LatestShort *SelfTestPollEntry `json:"latest_short,omitempty"`
	LatestLong  *SelfTestPollEntry `json:"latest_long,omitempty"`
}

type SelfTestPollEntry struct {
	Num           int    `json:"num"`
	Status        string `json:"status"`
	LifeTimeHours int    `json:"lifetime_hours"`
}

type selfTestRequest struct {
	Device string `json:"device"`
	Type   string `json:"type"`
}

type selfTestStarted struct {
	Device  string `json:"device"`
	Type    string `json:"type"`
	Started bool   `json:"started"`
}

type Server struct {
	addr             string
	mdDevice         string
	store            *status.Store
	onRefresh        RefreshFunc
	onSelfTest       SelfTestFunc
	onSelfTestStatus SelfTestStatusFunc
	refreshLock      sync.Mutex
	syncHub          *syncBroadcaster
	srv              *http.Server
}

func NewServer(addr string, mdDevice string, store *status.Store, onRefresh RefreshFunc, onSelfTest SelfTestFunc, onSelfTestStatus SelfTestStatusFunc) *Server {
	return &Server{
		addr:             addr,
		mdDevice:         mdDevice,
		store:            store,
		onRefresh:        onRefresh,
		onSelfTest:       onSelfTest,
		onSelfTestStatus: onSelfTestStatus,
		syncHub:          newSyncBroadcaster(store, mdDevice),
	}
}

func (s *Server) Start() error {
	page, err := template.ParseFS(pageFS, "templates/index.html")
	if err != nil {
		return fmt.Errorf("parse dashboard template: %w", err)
	}

	s.srv = &http.Server{
		Addr:              s.addr,
		Handler:           s.handler(page),
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

func (s *Server) handler(page *template.Template) http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(pageFS, "static")
	if err != nil {
		log.Printf("web: failed to load static assets: %v", err)
	} else {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

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
	mux.HandleFunc("/api/refresh", s.handleRefresh)
	mux.HandleFunc("/api/selftest", s.handleSelfTest)

	return mux
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
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
}

func (s *Server) handleSelfTest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.startSelfTest(w, r)
	case http.MethodGet:
		s.readSelfTest(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) startSelfTest(w http.ResponseWriter, r *http.Request) {
	if s.onSelfTest == nil {
		http.Error(w, "self-test not configured", http.StatusServiceUnavailable)
		return
	}

	var req selfTestRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid self-test request", http.StatusBadRequest)
		return
	}

	s.refreshLock.Lock()
	defer s.refreshLock.Unlock()

	if err := s.onSelfTest(req.Device, req.Type); err != nil {
		writeActionError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(selfTestStarted{
		Device:  req.Device,
		Type:    req.Type,
		Started: true,
	}); err != nil {
		log.Printf("web: failed to encode self-test response: %v", err)
	}
}

func (s *Server) readSelfTest(w http.ResponseWriter, r *http.Request) {
	if s.onSelfTestStatus == nil {
		http.Error(w, "self-test not configured", http.StatusServiceUnavailable)
		return
	}

	device := r.URL.Query().Get("device")
	poll, err := s.onSelfTestStatus(device)
	if err != nil {
		writeActionError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(poll); err != nil {
		log.Printf("web: failed to encode self-test status: %v", err)
	}
}

func writeActionError(w http.ResponseWriter, err error) {
	var statusErr StatusError
	if errors.As(err, &statusErr) && statusErr.Code != 0 {
		http.Error(w, statusErr.Msg, statusErr.Code)
		return
	}
	http.Error(w, err.Error(), http.StatusBadGateway)
}

func (s *Server) writeStatus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(s.store.Get()); err != nil {
		log.Printf("web: failed to encode status: %v", err)
	}
}
