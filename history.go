package main

import (
	"sync"
	"time"
	"unicode/utf8"
)

const previewMaxRunes = 100

// Entry is a single clipboard snapshot.
type Entry struct {
	Content   string
	Timestamp time.Time
}

// Preview returns a display-safe truncated string.
func (e Entry) Preview() string {
	s := e.Content
	if utf8.RuneCountInString(s) > previewMaxRunes {
		runes := []rune(s)
		return string(runes[:previewMaxRunes]) + "…"
	}
	return s
}

// HistoryStore holds clipboard entries newest-first with bounded size and age.
type HistoryStore struct {
	mu      sync.RWMutex
	entries []Entry
	maxAge  time.Duration
	maxSize int
}

func NewHistoryStore(maxAge time.Duration, maxSize int) *HistoryStore {
	return &HistoryStore{maxAge: maxAge, maxSize: maxSize}
}

// Add inserts content at the front, deduplicates, and evicts stale entries.
// Returns true when a new entry was recorded.
func (h *HistoryStore) Add(content string) bool {
	if content == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.entries) > 0 && h.entries[0].Content == content {
		return false
	}

	// Remove any existing duplicate deeper in the list so it bubbles to top.
	for i, e := range h.entries {
		if e.Content == content {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}

	h.entries = append([]Entry{{Content: content, Timestamp: time.Now()}}, h.entries...)
	h.evict()
	return true
}

func (h *HistoryStore) evict() {
	cutoff := time.Now().Add(-h.maxAge)
	n := len(h.entries)
	for i, e := range h.entries {
		if e.Timestamp.Before(cutoff) {
			n = i
			break
		}
	}
	if n > h.maxSize {
		n = h.maxSize
	}
	h.entries = h.entries[:n]
}

// All returns a snapshot of entries, newest first.
func (h *HistoryStore) All() []Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Entry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Clear removes all entries.
func (h *HistoryStore) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = nil
}
