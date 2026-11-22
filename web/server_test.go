package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	server := NewServer(":0", "/dev/md0", store, nil, nil, nil)
	handler := server.handler(nil)

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
			CheckedAt: time.Date(2026, 6, 20, 13, 0, 0, 0, time.UTC),
			MDDevice:  "/dev/md0",
			Healthy:   true,
			RAID:      status.RAIDStatus{Device: "/dev/md0", Healthy: true},
		})
	}, nil, nil)
	handler := server.handler(nil)

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

func TestStartSelfTest(t *testing.T) {
	var gotDevice, gotType string
	server := NewServer(":0", "/dev/md0", status.NewStore(), nil, func(device, testType string) error {
		gotDevice = device
		gotType = testType
		return nil
	}, func(device string) (SelfTestPoll, error) {
		return SelfTestPoll{Device: device, InProgress: true}, nil
	})
	handler := server.handler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/selftest", strings.NewReader(`{"device":"/dev/sdb1","type":"short"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotDevice != "/dev/sdb1" || gotType != "short" {
		t.Fatalf("started device=%q type=%q", gotDevice, gotType)
	}

	var started selfTestStarted
	if err := json.NewDecoder(rec.Body).Decode(&started); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !started.Started || started.Type != "short" {
		t.Fatalf("unexpected response: %+v", started)
	}
}

func TestStartSelfTestConflict(t *testing.T) {
	server := NewServer(":0", "/dev/md0", status.NewStore(), nil, func(device, testType string) error {
		return StatusError{Code: http.StatusConflict, Msg: "a self-test is already running on this disk"}
	}, nil)
	handler := server.handler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/selftest", strings.NewReader(`{"device":"/dev/sdb1","type":"short"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "already running") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestSelfTestStatus(t *testing.T) {
	server := NewServer(":0", "/dev/md0", status.NewStore(), nil, nil, func(device string) (SelfTestPoll, error) {
		if device != "/dev/sdb1" {
			t.Fatalf("unexpected device %q", device)
		}
		return SelfTestPoll{
			Device:     "/dev/sdb1",
			InProgress: false,
			LatestShort: &SelfTestPollEntry{
				Num:           1,
				Status:        "Completed without error",
				LifeTimeHours: 19670,
			},
		}, nil
	})
	handler := server.handler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/selftest?device=/dev/sdb1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var poll SelfTestPoll
	if err := json.NewDecoder(rec.Body).Decode(&poll); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if poll.InProgress || poll.LatestShort == nil || poll.LatestShort.LifeTimeHours != 19670 {
		t.Fatalf("unexpected poll: %+v", poll)
	}
}
