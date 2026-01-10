package main

import (
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

type ManualThrottle struct {
	mu          sync.RWMutex
	active      bool
	expiresAt   time.Time
	triggeredBy string
}

func NewManualThrottle() *ManualThrottle {
	return &ManualThrottle{}
}

func (m *ManualThrottle) Activate(duration time.Duration, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	m.expiresAt = time.Now().Add(duration)
	m.triggeredBy = username
}

func (m *ManualThrottle) Deactivate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	m.expiresAt = time.Time{}
	m.triggeredBy = ""
}

func (m *ManualThrottle) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.active {
		return false
	}
	return time.Now().Before(m.expiresAt)
}

func (m *ManualThrottle) TimeRemaining() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.active {
		return 0
	}
	remaining := time.Until(m.expiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (m *ManualThrottle) GetInfo() (bool, time.Time, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active && time.Now().Before(m.expiresAt), m.expiresAt, m.triggeredBy
}
