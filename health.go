package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type AppState struct {
	mu              sync.RWMutex
	state           State
	lastCheckTime   time.Time
	remoteStreams   int
	uploadLimitKbps int
	startTime       time.Time
}

func NewAppState() *AppState {
	return &AppState{
		state:     StateIdle,
		startTime: time.Now(),
	}
}

func (a *AppState) Update(state State, remoteStreams, uploadLimitKbps int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
	a.lastCheckTime = time.Now()
	a.remoteStreams = remoteStreams
	a.uploadLimitKbps = uploadLimitKbps
}

func (a *AppState) Get() (State, time.Time, int, int, time.Time) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state, a.lastCheckTime, a.remoteStreams, a.uploadLimitKbps, a.startTime
}

type ServiceHealth struct {
	Reachable bool  `json:"reachable"`
	LatencyMs int64 `json:"latency_ms"`
}

type HealthResponse struct {
	Status               string                   `json:"status"`
	State                string                   `json:"state"`
	UptimeSec            int64                    `json:"uptime_sec"`
	LastCheck            string                   `json:"last_check,omitempty"`
	RemoteStreams        int                      `json:"remote_streams"`
	CurrentUploadLimitKbps int                    `json:"current_upload_limit_kbps"`
	Services             map[string]ServiceHealth `json:"services"`
}

type HealthServer struct {
	port      int
	appState  *AppState
	plex      *PlexClient
	qbt       *QBittorrentClient
	eventCh   chan<- string
}

func NewHealthServer(port int, appState *AppState, plex *PlexClient, qbt *QBittorrentClient, eventCh chan<- string) *HealthServer {
	return &HealthServer{
		port:     port,
		appState: appState,
		plex:     plex,
		qbt:      qbt,
		eventCh:  eventCh,
	}
}

func (h *HealthServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/webhook", h.handleWebhook)

	addr := fmt.Sprintf(":%d", h.port)
	log.Printf("Starting server on %s (health + webhook)", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("Health server error: %v", err)
		}
	}()
}

func (h *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	state, lastCheck, remoteStreams, uploadLimit, startTime := h.appState.Get()

	services := make(map[string]ServiceHealth)

	plexStart := time.Now()
	_, plexErr := h.plex.GetRemoteStreamCount()
	plexLatency := time.Since(plexStart).Milliseconds()
	services["plex"] = ServiceHealth{
		Reachable: plexErr == nil,
		LatencyMs: plexLatency,
	}

	qbtStart := time.Now()
	qbtErr := h.qbt.Ping()
	qbtLatency := time.Since(qbtStart).Milliseconds()
	services["qbittorrent"] = ServiceHealth{
		Reachable: qbtErr == nil,
		LatencyMs: qbtLatency,
	}

	status := "healthy"
	statusCode := http.StatusOK
	if plexErr != nil || qbtErr != nil {
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	var lastCheckStr string
	if !lastCheck.IsZero() {
		lastCheckStr = lastCheck.Format(time.RFC3339)
	}

	resp := HealthResponse{
		Status:               status,
		State:                state.String(),
		UptimeSec:            int64(time.Since(startTime).Seconds()),
		LastCheck:            lastCheckStr,
		RemoteStreams:        remoteStreams,
		CurrentUploadLimitKbps: uploadLimit,
		Services:             services,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

type plexWebhookPayload struct {
	Event   string `json:"event"`
	Player  struct {
		Local bool `json:"local"`
	} `json:"Player"`
}

func (h *HealthServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	payload := r.FormValue("payload")
	if payload == "" {
		http.Error(w, "missing payload", http.StatusBadRequest)
		return
	}

	var webhook plexWebhookPayload
	if err := json.Unmarshal([]byte(payload), &webhook); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	switch webhook.Event {
	case "media.play", "media.resume", "media.stop", "media.pause":
		log.Printf("Webhook: %s (local=%v)", webhook.Event, webhook.Player.Local)
		select {
		case h.eventCh <- webhook.Event:
		default:
		}
	}

	w.WriteHeader(http.StatusOK)
}
