package service

import (
	"strings"
	"sync"
)

// Settings holds the LLM connection configuration. It is kept in process
// memory only - never written to SQLite or disk - regardless of whether it
// came from a CLI flag/env var or was set later from the browser (see
// docs/detailed-design.md §11). This tool runs on localhost for one
// operator at a time, so a single process-wide store (rather than a
// per-browser-session one) is a deliberate simplification: it avoids
// cookie/session plumbing while still meeting the "never persisted"
// requirement.
type Settings struct {
	APIKey  string
	Model   string
	BaseURL string
}

func (s Settings) Configured() bool {
	return s.BaseURL != "" && s.Model != ""
}

// MaskedAPIKey returns the key with everything but the last 4 characters
// replaced, for display in the UI. An empty key stays empty.
func (s Settings) MaskedAPIKey() string {
	if s.APIKey == "" {
		return ""
	}
	if len(s.APIKey) <= 4 {
		return strings.Repeat("*", len(s.APIKey))
	}
	return strings.Repeat("*", len(s.APIKey)-4) + s.APIKey[len(s.APIKey)-4:]
}

type SettingsStore struct {
	mu       sync.RWMutex
	settings Settings
}

func NewSettingsStore(initial Settings) *SettingsStore {
	return &SettingsStore{settings: initial}
}

func (s *SettingsStore) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// Update overwrites only the fields the caller set (non-empty values),
// leaving the rest as-is - so, e.g., updating the model alone doesn't
// clear a previously configured API key.
func (s *SettingsStore) Update(patch Settings) Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	if patch.APIKey != "" {
		s.settings.APIKey = patch.APIKey
	}
	if patch.Model != "" {
		s.settings.Model = patch.Model
	}
	if patch.BaseURL != "" {
		s.settings.BaseURL = patch.BaseURL
	}
	return s.settings
}
