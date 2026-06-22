package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ardean/mdadm-notifier/status"
)

func TestStatusEndpoint(t *testing.T) {
	store := status.NewStore()
	store.Update(status.Snapshot{
		Hostname:  "test-host",
		CheckedAt: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
		MDDevice:  "/dev/md0",
		Healthy:   true,
		Config: status.ConfigView{
			CheckInterval: "1h",
		},
		RAID: status.RAIDStatus{
			Device:  "/dev/md0",
			Healthy: true,
		},
	})

	server := NewServer(":0", "/dev/md0", store, nil)
	handler := newTestHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var snapshot status.Snapshot
	if err := json.NewDecoder(rec.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if snapshot.Hostname != "test-host" || snapshot.MDDevice != "/dev/md0" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestRefreshEndpoint(t *testing.T) {
	store := status.NewStore()
	var refreshCalls atomic.Int32

	server := NewServer(":0", "/dev/md0", store, func() {
		refreshCalls.Add(1)
		store.Update(status.Snapshot{
			Hostname:  "test-host",
			CheckedAt:   time.Date(2026, 6, 20, 13, 0, 0, 0, time.UTC),
			MDDevice:  "/dev/md0",
			Healthy:   true,
			RAID:      status.RAIDStatus{Device: "/dev/md0", Healthy: true},
		})
	})
	handler := newTestHandler(server)

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("expected refresh to run once, got %d", refreshCalls.Load())
	}

	var snapshot status.Snapshot
	if err := json.NewDecoder(rec.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if snapshot.CheckedAt.IsZero() {
		t.Fatal("expected refreshed snapshot")
	}
}

func newTestHandler(server *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		server.writeStatus(w)
	})
	mux.HandleFunc("/api/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if server.onRefresh == nil {
			http.Error(w, "refresh not configured", http.StatusServiceUnavailable)
			return
		}

		server.refreshLock.Lock()
		defer server.refreshLock.Unlock()
		server.onRefresh()
		server.writeStatus(w)
	})
	return mux
}
