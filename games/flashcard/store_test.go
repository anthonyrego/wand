package flashcard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// addCard is a test helper that runs a small image through the upload pipeline
// and adds it to the store, returning the new card's ID.
func addCard(t *testing.T, s *store, name string) string {
	t.Helper()
	d, png, err := processUpload(bytes.NewReader(makePNG(t, 8, 8)))
	if err != nil {
		t.Fatalf("processUpload: %v", err)
	}
	c, err := s.add(name, png, d)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	return c.ID
}

func ids(cards []Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.ID
	}
	return out
}

// firstPhoto returns the first photo of the card with the given ID.
func firstPhoto(t *testing.T, s *store, cardID string) Photo {
	t.Helper()
	for _, c := range s.cards {
		if c.ID == cardID {
			if len(c.Photos) == 0 {
				t.Fatalf("card %s has no photos", cardID)
			}
			return c.Photos[0]
		}
	}
	t.Fatalf("card %s not found", cardID)
	return Photo{}
}

func TestStoreAddListAndAutoCurrent(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	a := addCard(t, s, "Apple")
	b := addCard(t, s, "Banana")

	cards, current, _ := s.list()
	if len(cards) != 2 {
		t.Fatalf("want 2 cards, got %d", len(cards))
	}
	if current != a {
		t.Fatalf("first added card should be current; want %s got %s", a, current)
	}
	if cards[0].ID != a || cards[1].ID != b {
		t.Fatalf("order wrong: %v", ids(cards))
	}
}

func TestStoreSetCurrentAndStep(t *testing.T) {
	s, _ := newStore(t.TempDir())
	a := addCard(t, s, "A")
	b := addCard(t, s, "B")
	c := addCard(t, s, "C")

	s.setCurrent(b)
	if _, cur, _ := s.list(); cur != b {
		t.Fatalf("setCurrent failed: %s", cur)
	}

	s.step(+1)
	if _, cur, _ := s.list(); cur != c {
		t.Fatalf("step +1 want %s got %s", c, cur)
	}

	s.step(+1) // wrap to first
	if _, cur, _ := s.list(); cur != a {
		t.Fatalf("step wrap want %s got %s", a, cur)
	}

	s.step(-1) // wrap back to last
	if _, cur, _ := s.list(); cur != c {
		t.Fatalf("step -1 wrap want %s got %s", c, cur)
	}
}

func TestStoreMoveReorders(t *testing.T) {
	s, _ := newStore(t.TempDir())
	a := addCard(t, s, "A")
	b := addCard(t, s, "B")
	c := addCard(t, s, "C")

	s.move(a, 2) // A to the end
	cards, _, _ := s.list()
	if got := ids(cards); got[0] != b || got[1] != c || got[2] != a {
		t.Fatalf("after move: %v", got)
	}

	s.move(c, 0) // C to the front
	cards, _, _ = s.list()
	if got := ids(cards); got[0] != c || got[1] != b || got[2] != a {
		t.Fatalf("after second move: %v", got)
	}
}

func TestStoreRemoveAdvancesCurrentAndDeletesFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := newStore(dir)
	a := addCard(t, s, "A")
	b := addCard(t, s, "B")

	// The image file for A's photo should exist on disk.
	file := filepath.Join(dir, firstPhoto(t, s, a).File)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("image file missing: %v", err)
	}

	s.remove(a) // removing the current card advances to b
	if _, cur, _ := s.list(); cur != b {
		t.Fatalf("current should advance to %s, got %s", b, cur)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("image file should be deleted, stat err = %v", err)
	}

	s.remove(b) // removing the last card clears current
	cards, cur, _ := s.list()
	if len(cards) != 0 || cur != "" {
		t.Fatalf("deck should be empty with no current; cards=%d cur=%q", len(cards), cur)
	}
}

func TestStoreRenamePersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	s, _ := newStore(dir)
	a := addCard(t, s, "Apple")
	_ = addCard(t, s, "Banana")
	s.rename(a, "Avocado")
	s.setCurrent(a)

	// Reopen from disk — manifest + images should reload.
	s2, err := newStore(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cards, cur, _ := s2.list()
	if len(cards) != 2 {
		t.Fatalf("reloaded deck size = %d, want 2", len(cards))
	}
	if cards[0].Name != "Avocado" {
		t.Fatalf("rename did not persist: %q", cards[0].Name)
	}
	if cur != a {
		t.Fatalf("current did not persist: %q", cur)
	}
	// Every reloaded card should have pending pixels ready to upload.
	if len(s2.pending) != 2 {
		t.Fatalf("pending images = %d, want 2", len(s2.pending))
	}
}

func TestStoreMigratesLegacySingleImageManifest(t *testing.T) {
	dir := t.TempDir()

	// Hand-write a pre-multi-photo manifest: a card with the old top-level
	// "file" field and no "photos" array, plus its image on disk.
	if err := os.WriteFile(filepath.Join(dir, "legacy.png"), makePNG(t, 8, 8), 0o644); err != nil {
		t.Fatal(err)
	}
	old := `{"cards":[{"id":"card-1","name":"Apple","file":"legacy.png"}],"currentId":"card-1"}`
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	cards, cur, photo := s.list()
	if len(cards) != 1 || cur != "card-1" || photo != 0 {
		t.Fatalf("migrated deck wrong: cards=%d cur=%q photo=%d", len(cards), cur, photo)
	}
	if len(cards[0].Photos) != 1 || cards[0].Photos[0].File != "legacy.png" {
		t.Fatalf("legacy file not migrated to a photo: %+v", cards[0].Photos)
	}
	if cards[0].File != "" {
		t.Fatalf("legacy File field should be cleared in memory, got %q", cards[0].File)
	}

	// A mutation rewrites the manifest in the new format: the card-level legacy
	// "file" is gone (omitempty) and the photo lives under "photos".
	s.rename("card-1", "Avocado")
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("reparse manifest: %v", err)
	}
	if len(m.Cards) != 1 || m.Cards[0].File != "" || len(m.Cards[0].Photos) != 1 {
		t.Fatalf("rewritten manifest still legacy-shaped: %s", data)
	}
}

func TestSnapshotIfChanged(t *testing.T) {
	s, _ := newStore(t.TempDir())

	// Fresh store starts at revision 1, so seen=0 reports changed.
	snap, changed := s.snapshotIfChanged(0)
	if !changed {
		t.Fatal("expected initial change")
	}
	seen := snap.revision

	if _, changed := s.snapshotIfChanged(seen); changed {
		t.Fatal("expected no change when revision unchanged")
	}

	id := addCard(t, s, "A")
	photoID := firstPhoto(t, s, id).ID
	snap, changed = s.snapshotIfChanged(seen)
	if !changed {
		t.Fatal("expected change after add")
	}
	if snap.uploads[photoID] == nil {
		t.Fatal("new card's photo should appear in uploads")
	}

	// Uploads are popped, so a subsequent snapshot (after another change) won't
	// re-deliver the same pixels.
	s.rename(id, "B")
	snap, _ = s.snapshotIfChanged(snap.revision)
	if _, ok := snap.uploads[photoID]; ok {
		t.Fatal("uploads should be delivered exactly once")
	}
}
