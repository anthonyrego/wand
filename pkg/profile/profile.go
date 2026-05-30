// Package profile stores the wand's personalization — the owner's name — and
// derives the on-screen title from it ("EMMA'S WAND"). The name persists to
// ~/.wand/profile.json and is editable at runtime from any device on the LAN
// via the browser-based admin in server.go, mirroring the flashcard deck's
// manage-from-your-phone pattern.
//
// The Store is the single source of truth, shared between the HTTP server
// (which writes the name) and the game-loop selector (which reads it). A
// revision counter lets the selector cheaply detect edits and rebuild its
// title text live.
package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	fileName    = "profile.json"
	maxNameLen  = 24   // keeps the title readable and on-screen in the 8x8 font
	DefaultPort = 8090 // distinct from the flashcard admin (8080)
)

// Store holds the owner name and persists it to disk. All access is guarded by
// mu; revision bumps on every change so readers can detect updates.
type Store struct {
	mu       sync.Mutex
	path     string // "" means in-memory only (e.g. when the dir couldn't be made)
	name     string
	revision uint64
}

type persisted struct {
	OwnerName string `json:"ownerName"`
}

// Load opens (creating its directory if needed) the profile at dir/profile.json.
// It always returns a usable Store: a missing or unreadable file yields an empty
// name (the plain "WAND" title), and a directory error returns an in-memory-only
// store alongside the error so the caller can warn but keep running.
func Load(dir string) (*Store, error) {
	s := &Store{revision: 1}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return s, err
	}
	s.path = filepath.Join(dir, fileName)

	data, err := os.ReadFile(s.path)
	if err != nil {
		return s, nil // fresh profile; missing file is expected
	}
	var p persisted
	if err := json.Unmarshal(data, &p); err == nil {
		s.name = CleanName(p.OwnerName)
	}
	return s, nil
}

// Name returns the raw owner name (e.g. "Emma"), or "" if unset.
func (s *Store) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Revision returns a counter that changes whenever the name changes.
func (s *Store) Revision() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

// Title returns the personalized selector title in the all-caps style of the
// rest of the UI: "EMMA'S WAND", or plain "WAND" when no name is set.
func (s *Store) Title() string {
	return Title(s.Name())
}

// SetName cleans, stores, and persists a new owner name, returning the cleaned
// value actually stored. It's a no-op (no revision bump) if the name is unchanged.
func (s *Store) SetName(name string) string {
	name = CleanName(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == s.name {
		return s.name
	}
	s.name = name
	s.revision++
	s.saveLocked()
	return s.name
}

func (s *Store) saveLocked() {
	if s.path == "" {
		return
	}
	data, err := json.MarshalIndent(persisted{OwnerName: s.name}, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

// Title derives the all-caps selector title from a raw owner name.
func Title(name string) string {
	name = CleanName(name)
	if name == "" {
		return "WAND"
	}
	return strings.ToUpper(name) + "'S WAND"
}

// WindowTitle derives the OS window title in proper case: "Emma's Wand".
func WindowTitle(name string) string {
	name = CleanName(name)
	if name == "" {
		return "Wand"
	}
	return name + "'s Wand"
}

// CleanName trims a name and restricts it to the printable ASCII the 8x8 font
// covers, capped at maxNameLen so the title stays on screen.
func CleanName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		if r >= 0x20 && r <= 0x7E { // printable ASCII covered by pkg/sign
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > maxNameLen { // out is ASCII, so bytes == runes
		out = strings.TrimSpace(out[:maxNameLen])
	}
	return out
}

// Dir is the profile directory: $WAND_CONFIG_DIR, else ~/.wand (the same root
// the flashcard deck uses).
func Dir() string {
	if d := os.Getenv("WAND_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".wand"
	}
	return filepath.Join(home, ".wand")
}

// Port is the admin server port: $WAND_CONFIG_PORT, else DefaultPort.
func Port() int {
	if p := os.Getenv("WAND_CONFIG_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return DefaultPort
}
