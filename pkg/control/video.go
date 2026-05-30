package control

import (
	"sync"

	"github.com/anthonyrego/toybox/pkg/settings"
)

// Resolution is one selectable window size, served to the web UI. The list is
// snapshotted once at startup (the window enumerates display modes on the main
// thread), so handlers never touch SDL.
type Resolution struct {
	W int `json:"w"`
	H int `json:"h"`
}

// VideoStore holds the display settings, shared between the web (which writes a
// full payload on save) and the game loop (which applies changes the next
// frame). It persists to disk on every change. All access is guarded by mu;
// revision bumps on change so the loop can cheaply detect edits.
type VideoStore struct {
	mu       sync.Mutex
	path     string // "" = in-memory only
	s        settings.Settings
	revision uint64
}

// NewVideoStore loads the settings at path (falling back to defaults) and
// returns a store that will persist future changes back to it.
func NewVideoStore(path string) *VideoStore {
	return &VideoStore{path: path, s: settings.Load(path), revision: 1}
}

// Settings returns the current display settings.
func (v *VideoStore) Settings() settings.Settings {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.s
}

// Revision returns the current change counter, so a caller can prime its
// "seen" cursor (e.g. at startup) without applying anything.
func (v *VideoStore) Revision() uint64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.revision
}

// Set sanitizes, stores, persists, and bumps the revision, returning the stored
// value. Called from the HTTP handler (CPU/disk only — never touches SDL/GPU).
func (v *VideoStore) Set(s settings.Settings) settings.Settings {
	s = settings.Sanitize(s)
	v.mu.Lock()
	defer v.mu.Unlock()
	v.s = s
	v.revision++
	if v.path != "" {
		_ = settings.Save(v.path, s)
	}
	return s
}

// SettingsIfChanged returns the current settings only when the revision differs
// from seen; otherwise ok is false and the caller skips work.
func (v *VideoStore) SettingsIfChanged(seen uint64) (s settings.Settings, rev uint64, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.revision == seen {
		return settings.Settings{}, seen, false
	}
	return v.s, v.revision, true
}
