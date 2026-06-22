package web

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/status"
	"github.com/gorilla/websocket"
)

const defaultSyncPollInterval = time.Second

type syncMessage struct {
	Sync      *mdadm.SyncProgress `json:"sync"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type syncBroadcaster struct {
	mu       sync.Mutex
	clients  map[*websocket.Conn]struct{}
	store    *status.Store
	device   string
	interval time.Duration

	pollStop    chan struct{}
	pollRunning bool
	last        mdadm.SyncProgress
}

func newSyncBroadcaster(store *status.Store, device string) *syncBroadcaster {
	return &syncBroadcaster{
		store:    store,
		device:   device,
		interval: defaultSyncPollInterval,
	}
}

var syncUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (b *syncBroadcaster) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := syncUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("web: sync websocket upgrade failed: %v", err)
		return
	}

	b.subscribe(conn)
	defer func() {
		b.unsubscribe(conn)
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (b *syncBroadcaster) subscribe(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.clients == nil {
		b.clients = make(map[*websocket.Conn]struct{})
	}
	b.clients[conn] = struct{}{}

	if msg, ok := b.readAndStoreLocked(); ok {
		b.write(conn, msg)
	}

	if !b.pollRunning {
		b.startPollerLocked()
	}
}

func (b *syncBroadcaster) unsubscribe(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.clients, conn)
	if len(b.clients) == 0 && b.pollRunning {
		b.stopPollerLocked()
	}
}

func (b *syncBroadcaster) startPollerLocked() {
	b.pollStop = make(chan struct{})
	b.pollRunning = true

	go func() {
		ticker := time.NewTicker(b.interval)
		defer ticker.Stop()

		for {
			select {
			case <-b.pollStop:
				return
			case <-ticker.C:
				b.mu.Lock()
				if len(b.clients) == 0 {
					b.stopPollerLocked()
					b.mu.Unlock()
					return
				}

				msg, ok := b.readAndStoreLocked()
				if ok && syncProgressChanged(b.last, *msg.Sync) {
					b.broadcastLocked(msg)
				}
				b.mu.Unlock()
			}
		}
	}()
}

func (b *syncBroadcaster) stopPollerLocked() {
	if !b.pollRunning {
		return
	}
	close(b.pollStop)
	b.pollRunning = false
}

func (b *syncBroadcaster) readAndStoreLocked() (syncMessage, bool) {
	detail := b.store.RAIDDetail()
	progress, err := mdadm.ReadSyncProgress(b.device, detail)
	if err != nil {
		log.Printf("web: sync progress read failed: %v", err)
		return syncMessage{}, false
	}

	b.store.UpdateSync(progress)
	b.last = *progress

	return syncMessage{
		Sync:      progress,
		UpdatedAt: time.Now().UTC(),
	}, true
}

func (b *syncBroadcaster) broadcastLocked(msg syncMessage) {
	for conn := range b.clients {
		b.write(conn, msg)
	}
}

func (b *syncBroadcaster) write(conn *websocket.Conn, msg syncMessage) {
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("web: sync websocket write failed: %v", err)
	}
}

func syncProgressChanged(a, b mdadm.SyncProgress) bool {
	return a.Active != b.Active ||
		a.Action != b.Action ||
		a.Percent != b.Percent ||
		a.Completed != b.Completed ||
		a.Total != b.Total ||
		a.FinishMinutes != b.FinishMinutes ||
		a.SpeedKBps != b.SpeedKBps ||
		a.Pending != b.Pending ||
		a.Delayed != b.Delayed
}

func (s *Server) writeSync(w http.ResponseWriter) {
	detail := s.store.RAIDDetail()
	progress, err := mdadm.ReadSyncProgress(s.mdDevice, detail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.store.UpdateSync(progress)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(syncMessage{
		Sync:      progress,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		log.Printf("web: failed to encode sync status: %v", err)
	}
}
