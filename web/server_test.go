package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	server := NewServer(":0", store)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(store.Get())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

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

	_ = server
}
