package flashcard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

const manifestName = "manifest.json"

// Card is one flashcard's persisted metadata. The image lives in a separate
// file (File) alongside the manifest.
type Card struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// manifest is the on-disk JSON describing the whole deck.
type manifest struct {
	Cards     []Card `json:"cards"`
	CurrentID string `json:"currentId"`
}

// store is the source of truth for the deck, shared between the HTTP server
// (which mutates it) and the game loop (which reads snapshots). It only ever
// holds CPU-side data — never GPU resources. All access is guarded by mu.
//
// pending holds decoded RGBA for cards the game loop has not yet uploaded to
// the GPU; the game loop pops entries as it creates textures. revision is
// bumped on every mutation so the game loop can cheaply detect changes.
type store struct {
	mu        sync.Mutex
	dir       string
	cards     []Card
	currentID string
	pending   map[string]*decoded
	revision  uint64
}

// newStore opens (creating if needed) the deck directory, loads the manifest,
// and decodes every referenced image into the pending map so the game loop
// uploads them on its first frames.
func newStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create deck dir: %w", err)
	}

	s := &store{dir: dir, pending: map[string]*decoded{}, revision: 1}

	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil // fresh deck
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Keep only cards whose image file decodes successfully.
	for _, c := range m.Cards {
		b, err := os.ReadFile(filepath.Join(dir, c.File))
		if err != nil {
			continue
		}
		d, err := decodeStored(bytes.NewReader(b))
		if err != nil {
			continue
		}
		s.cards = append(s.cards, c)
		s.pending[c.ID] = d
	}

	s.currentID = m.CurrentID
	s.fixCurrentLocked()
	return s, nil
}

// snapshot is a lock-free copy of what the game loop needs each frame, plus any
// newly decoded images to upload (popped from pending so they're handed off
// exactly once).
type snapshot struct {
	revision  uint64
	cards     []Card
	currentID string
	uploads   map[string]*decoded
}

// snapshotIfChanged returns a snapshot only when the deck has changed since the
// caller's last seen revision; otherwise ok is false and the caller skips work.
func (s *store) snapshotIfChanged(seen uint64) (snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.revision == seen {
		return snapshot{}, false
	}

	cards := make([]Card, len(s.cards))
	copy(cards, s.cards)

	var uploads map[string]*decoded
	for _, c := range cards {
		if d := s.pending[c.ID]; d != nil {
			if uploads == nil {
				uploads = map[string]*decoded{}
			}
			uploads[c.ID] = d
			delete(s.pending, c.ID)
		}
	}

	return snapshot{
		revision:  s.revision,
		cards:     cards,
		currentID: s.currentID,
		uploads:   uploads,
	}, true
}

// list returns a copy of the deck and the current card ID, for the HTTP API.
func (s *store) list() ([]Card, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cards := make([]Card, len(s.cards))
	copy(cards, s.cards)
	return cards, s.currentID
}

// fileForID returns the image filename for a card, for serving over HTTP.
func (s *store) fileForID(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cards {
		if c.ID == id {
			return c.File, true
		}
	}
	return "", false
}

// add writes the image to disk and appends a new card. The first card added
// becomes the current (on-screen) card automatically.
func (s *store) add(name string, png []byte, d *decoded) (Card, error) {
	id := uuid.NewString()
	file := id + ".png"
	if err := os.WriteFile(filepath.Join(s.dir, file), png, 0o644); err != nil {
		return Card{}, fmt.Errorf("write image: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c := Card{ID: id, Name: name, File: file}
	s.cards = append(s.cards, c)
	s.pending[id] = d
	if s.currentID == "" {
		s.currentID = id
	}
	s.touchLocked()
	return c, nil
}

// remove deletes a card and its image file. If it was the current card, the
// current selection advances to the next remaining card.
func (s *store) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.indexLocked(id)
	if idx < 0 {
		return
	}
	file := s.cards[idx].File
	s.cards = append(s.cards[:idx], s.cards[idx+1:]...)
	delete(s.pending, id)

	if s.currentID == id {
		switch {
		case len(s.cards) == 0:
			s.currentID = ""
		case idx < len(s.cards):
			s.currentID = s.cards[idx].ID
		default:
			s.currentID = s.cards[len(s.cards)-1].ID
		}
	}

	_ = os.Remove(filepath.Join(s.dir, file))
	s.touchLocked()
}

// rename changes a card's display name.
func (s *store) rename(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx := s.indexLocked(id); idx >= 0 {
		s.cards[idx].Name = name
		s.touchLocked()
	}
}

// move relocates a card to a new position in the deck order, clamping to range.
func (s *store) move(id string, to int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	from := s.indexLocked(id)
	if from < 0 {
		return
	}
	if to < 0 {
		to = 0
	}
	if to >= len(s.cards) {
		to = len(s.cards) - 1
	}
	if to == from {
		return
	}

	c := s.cards[from]
	s.cards = append(s.cards[:from], s.cards[from+1:]...)
	s.cards = append(s.cards[:to], append([]Card{c}, s.cards[to:]...)...)
	s.touchLocked()
}

// setCurrent selects the on-screen card by ID (no-op if unknown).
func (s *store) setCurrent(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexLocked(id) >= 0 {
		s.currentID = id
		s.touchLocked()
	}
}

// step advances the current selection by delta (+1 / -1), wrapping around.
func (s *store) step(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.cards)
	if n == 0 {
		return
	}
	idx := s.indexLocked(s.currentID)
	if idx < 0 {
		idx = 0
	} else {
		idx = ((idx+delta)%n + n) % n
	}
	s.currentID = s.cards[idx].ID
	s.touchLocked()
}

// --- helpers (caller must hold mu) ---

func (s *store) indexLocked(id string) int {
	for i, c := range s.cards {
		if c.ID == id {
			return i
		}
	}
	return -1
}

func (s *store) fixCurrentLocked() {
	if s.currentID != "" && s.indexLocked(s.currentID) < 0 {
		s.currentID = ""
	}
	if s.currentID == "" && len(s.cards) > 0 {
		s.currentID = s.cards[0].ID
	}
}

// touchLocked bumps the revision and persists the manifest to disk.
func (s *store) touchLocked() {
	s.revision++
	m := manifest{Cards: s.cards, CurrentID: s.currentID}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	tmp := filepath.Join(s.dir, manifestName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(s.dir, manifestName))
}
