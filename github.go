package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// PRState represents the state of a GitHub PR
type PRState struct {
	Merged bool
	Closed bool
	Open   bool
}

// PRCacheEntry stores a PR state with expiration
type PRCacheEntry struct {
	State      PRState
	ExpiresAt  time.Time
}

// PRCache caches GitHub PR states with 60-second TTL
type PRCache struct {
	mu      sync.RWMutex
	entries map[string]PRCacheEntry // key: "org/repo/pr"
}

func newPRCache() *PRCache {
	return &PRCache{
		entries: make(map[string]PRCacheEntry),
	}
}

func (c *PRCache) get(org, repo string, pr int) (*PRState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s/%d", org, repo, pr)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return &entry.State, true
}

func (c *PRCache) set(org, repo string, pr int, state PRState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s/%d", org, repo, pr)
	c.entries[key] = PRCacheEntry{
		State:     state,
		ExpiresAt: time.Now().Add(60 * time.Second),
	}
}

// checkPRState checks the state of a GitHub PR using gh CLI
func checkPRState(org, repo string, pr int) (*PRState, error) {
	// Use gh api to get PR state
	// gh api repos/{owner}/{repo}/pulls/{pull_number}
	cmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/%s/pulls/%d", org, repo, pr))
	output, err := cmd.Output()
	if err != nil {
		// If gh command fails, return error
		return nil, fmt.Errorf("gh api failed: %w", err)
	}

	// Parse JSON response
	var prData struct {
		State  string `json:"state"`  // "open" or "closed"
		Merged bool   `json:"merged"` // true if merged
	}
	if err := json.Unmarshal(output, &prData); err != nil {
		return nil, fmt.Errorf("failed to parse gh api response: %w", err)
	}

	state := PRState{
		Merged: prData.Merged,
		Closed: prData.State == "closed",
		Open:   prData.State == "open",
	}

	return &state, nil
}
