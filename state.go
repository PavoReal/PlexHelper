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
