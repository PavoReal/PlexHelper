package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type CooldownTracker struct {
	mu                sync.Mutex
	transitions       []time.Time
	maxTransitions    int
	windowDuration    time.Duration
	statePath         string
}

type cooldownState struct {
	Transitions []time.Time `json:"transitions"`
}

func NewCooldownTracker(maxTransitions, windowMinutes int, statePath string) *CooldownTracker {
	ct := &CooldownTracker{
		maxTransitions: maxTransitions,
		windowDuration: time.Duration(windowMinutes) * time.Minute,
		statePath:      statePath,
	}
	ct.load()
	return ct
}

func (ct *CooldownTracker) CanTransitionToIdle() bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.pruneExpired()
	return len(ct.transitions) < ct.maxTransitions
}

func (ct *CooldownTracker) RecordTransition() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.transitions = append(ct.transitions, time.Now())
	ct.save()
}

func (ct *CooldownTracker) TransitionsInWindow() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.pruneExpired()
	return len(ct.transitions)
}

func (ct *CooldownTracker) pruneExpired() {
	cutoff := time.Now().Add(-ct.windowDuration)
	valid := ct.transitions[:0]
	for _, t := range ct.transitions {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	ct.transitions = valid
}

func (ct *CooldownTracker) load() {
	data, err := os.ReadFile(ct.statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: failed to read cooldown state: %v", err)
		}
		return
	}

	var state cooldownState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("Warning: failed to parse cooldown state: %v", err)
		return
	}

	ct.mu.Lock()
	ct.transitions = state.Transitions
	ct.pruneExpired()
	ct.mu.Unlock()

	if len(ct.transitions) > 0 {
		log.Printf("Loaded %d recent cooldown transitions from state file", len(ct.transitions))
	}
}

func (ct *CooldownTracker) save() {
	state := cooldownState{Transitions: ct.transitions}
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("Warning: failed to marshal cooldown state: %v", err)
		return
	}

	if err := os.WriteFile(ct.statePath, data, 0644); err != nil {
		log.Printf("Warning: failed to save cooldown state: %v", err)
	}
}
