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

// Photo is one image belonging to a card. The pixels live in a separate file
// (File) alongside the manifest; ID keys both the persisted reference and the
// game loop's GPU texture for this image.
type Photo struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// Card is one flashcard's persisted metadata. A card has a name and one or more
// photos; whichever photo is selected is shown on screen, and a parent can
// cycle through them.
type Card struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Photos []Photo `json:"photos"`

	// File is a legacy single-image reference from pre-multi-photo manifests.
	// It is migrated into Photos on load and never written back (omitempty,
	// and cleared in memory after migration).
	File string `json:"file,omitempty"`
}

// manifest is the on-disk JSON describing the whole deck.
type manifest struct {
	Cards        []Card `json:"cards"`
	CurrentID    string `json:"currentId"`
	CurrentPhoto int    `json:"currentPhoto"`
}

// store is the source of truth for the deck, shared between the HTTP server
// (which mutates it) and the game loop (which reads snapshots). It only ever
// holds CPU-side data — never GPU resources. All access is guarded by mu.
//
// pending holds decoded RGBA for photos the game loop has not yet uploaded to
// the GPU, keyed by photo ID; the game loop pops entries as it creates
// textures. revision is bumped on every mutation so the game loop can cheaply
// detect changes. currentID names the on-screen card and currentPhoto indexes
// into that card's Photos.
type store struct {
	mu           sync.Mutex
	dir          string
	cards        []Card
	currentID    string
	currentPhoto int
	pending      map[string]*decoded
	revision     uint64
}

// newStore opens (creating if needed) the deck directory, loads the manifest,
// and decodes every referenced image into the pending map so the game loop
// uploads them on its first frames. Legacy single-image cards are migrated to
// the multi-photo model in memory.
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

	for _, c := range m.Cards {
		// Migrate legacy single-image cards into the photos model.
		if len(c.Photos) == 0 && c.File != "" {
			c.Photos = []Photo{{ID: uuid.NewString(), File: c.File}}
		}
		c.File = "" // never persist the legacy field again

		// Keep only photos whose image file decodes successfully, and only keep
		// the card if at least one photo survives.
		var photos []Photo
		for _, p := range c.Photos {
			b, err := os.ReadFile(filepath.Join(dir, p.File))
			if err != nil {
				continue
			}
			d, err := decodeStored(bytes.NewReader(b))
			if err != nil {
				continue
			}
			photos = append(photos, p)
			s.pending[p.ID] = d
		}
		if len(photos) == 0 {
			continue
		}
		c.Photos = photos
		s.cards = append(s.cards, c)
	}

	s.currentID = m.CurrentID
	s.currentPhoto = m.CurrentPhoto
	s.fixCurrentLocked()
	return s, nil
}

// snapshot is a lock-free copy of what the game loop needs each frame, plus any
// newly decoded images to upload (popped from pending so they're handed off
// exactly once). uploads is keyed by photo ID.
type snapshot struct {
	revision     uint64
	cards        []Card
	currentID    string
	currentPhoto int
	uploads      map[string]*decoded
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
		for _, p := range c.Photos {
			if d := s.pending[p.ID]; d != nil {
				if uploads == nil {
					uploads = map[string]*decoded{}
				}
				uploads[p.ID] = d
				delete(s.pending, p.ID)
			}
		}
	}

	return snapshot{
		revision:     s.revision,
		cards:        cards,
		currentID:    s.currentID,
		currentPhoto: s.currentPhoto,
		uploads:      uploads,
	}, true
}

// list returns a copy of the deck, the current card ID, and the current photo
// index within that card, for the HTTP API.
func (s *store) list() ([]Card, string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cards := make([]Card, len(s.cards))
	copy(cards, s.cards)
	return cards, s.currentID, s.currentPhoto
}

// fileForPhoto returns the image filename for a specific photo of a card, for
// serving over HTTP.
func (s *store) fileForPhoto(cardID, photoID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.cards {
		if c.ID != cardID {
			continue
		}
		for _, p := range c.Photos {
			if p.ID == photoID {
				return p.File, true
			}
		}
	}
	return "", false
}

// add writes the image to disk and appends a new single-photo card. The first
// card added becomes the current (on-screen) card automatically.
func (s *store) add(name string, png []byte, d *decoded) (Card, error) {
	photoID := uuid.NewString()
	file := photoID + ".png"
	if err := os.WriteFile(filepath.Join(s.dir, file), png, 0o644); err != nil {
		return Card{}, fmt.Errorf("write image: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c := Card{ID: uuid.NewString(), Name: name, Photos: []Photo{{ID: photoID, File: file}}}
	s.cards = append(s.cards, c)
	s.pending[photoID] = d
	if s.currentID == "" {
		s.currentID = c.ID
		s.currentPhoto = 0
	}
	s.touchLocked()
	return c, nil
}

// addPhoto writes the image to disk and appends it to an existing card. It is a
// no-op if the card is unknown.
func (s *store) addPhoto(cardID string, png []byte, d *decoded) (Photo, error) {
	photoID := uuid.NewString()
	file := photoID + ".png"

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.indexLocked(cardID)
	if idx < 0 {
		return Photo{}, fmt.Errorf("unknown card")
	}
	if err := os.WriteFile(filepath.Join(s.dir, file), png, 0o644); err != nil {
		return Photo{}, fmt.Errorf("write image: %w", err)
	}

	p := Photo{ID: photoID, File: file}
	s.cards[idx].Photos = append(s.cards[idx].Photos, p)
	s.pending[photoID] = d
	s.touchLocked()
	return p, nil
}

// remove deletes a card and all its image files. If it was the current card, the
// current selection advances to the next remaining card.
func (s *store) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.indexLocked(id)
	if idx < 0 {
		return
	}
	files := make([]string, 0, len(s.cards[idx].Photos))
	for _, p := range s.cards[idx].Photos {
		files = append(files, p.File)
		delete(s.pending, p.ID)
	}
	s.cards = append(s.cards[:idx], s.cards[idx+1:]...)

	if s.currentID == id {
		switch {
		case len(s.cards) == 0:
			s.currentID = ""
		case idx < len(s.cards):
			s.currentID = s.cards[idx].ID
		default:
			s.currentID = s.cards[len(s.cards)-1].ID
		}
		s.currentPhoto = 0
	}

	for _, f := range files {
		_ = os.Remove(filepath.Join(s.dir, f))
	}
	s.touchLocked()
}

// removePhoto deletes one photo from a card and its image file. It is a no-op
// when the photo is the card's only one (use remove to delete the whole card),
// keeping the invariant that every card has at least one photo.
func (s *store) removePhoto(cardID, photoID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ci := s.indexLocked(cardID)
	if ci < 0 || len(s.cards[ci].Photos) <= 1 {
		return
	}
	photos := s.cards[ci].Photos
	pi := -1
	for i, p := range photos {
		if p.ID == photoID {
			pi = i
			break
		}
	}
	if pi < 0 {
		return
	}

	file := photos[pi].File
	s.cards[ci].Photos = append(photos[:pi], photos[pi+1:]...)
	delete(s.pending, photoID)

	// Keep the on-screen photo pointing at the same image where possible.
	if s.currentID == cardID && pi <= s.currentPhoto && s.currentPhoto > 0 {
		s.currentPhoto--
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

// setCurrent selects the on-screen card by ID (no-op if unknown), resetting the
// photo selection to the card's first photo.
func (s *store) setCurrent(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexLocked(id) >= 0 {
		s.currentID = id
		s.currentPhoto = 0
		s.touchLocked()
	}
}

// selectPhoto makes a specific photo of a card the on-screen one, selecting the
// card too. The index is clamped into range; unknown cards are a no-op.
func (s *store) selectPhoto(cardID string, photo int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.indexLocked(cardID) < 0 {
		return
	}
	s.currentID = cardID
	s.currentPhoto = s.clampPhotoLocked(cardID, photo)
	s.touchLocked()
}

// step advances the current card selection by delta (+1 / -1), wrapping around,
// and resets to the new card's first photo.
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
	s.currentPhoto = 0
	s.touchLocked()
}

// stepPhoto cycles the on-screen photo within the current card by delta
// (+1 / -1), wrapping around.
func (s *store) stepPhoto(delta int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.indexLocked(s.currentID)
	if idx < 0 {
		return
	}
	n := len(s.cards[idx].Photos)
	if n == 0 {
		return
	}
	s.currentPhoto = ((s.currentPhoto+delta)%n + n) % n
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
	// Clamp the photo index into the current card's range.
	s.currentPhoto = s.clampPhotoLocked(s.currentID, s.currentPhoto)
}

// clampPhotoLocked returns idx bounded to the card's photo range (0 when the
// card is unknown or empty).
func (s *store) clampPhotoLocked(cardID string, idx int) int {
	ci := s.indexLocked(cardID)
	if ci < 0 || len(s.cards[ci].Photos) == 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if max := len(s.cards[ci].Photos) - 1; idx > max {
		return max
	}
	return idx
}

// touchLocked bumps the revision and persists the manifest to disk.
func (s *store) touchLocked() {
	s.revision++
	m := manifest{Cards: s.cards, CurrentID: s.currentID, CurrentPhoto: s.currentPhoto}
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
