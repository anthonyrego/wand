package flashcard

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
)

const (
	maxUploadBytes = 15 << 20 // 15 MiB ceiling on a single upload
	maxNameLen     = 40
)

// module exposes the deck over HTTP as a control.GameModule: the deck-management
// SPA (Page, mounted at /game/) and its REST (API, mounted at /api/game/). It
// only ever touches the store (CPU-side); no GPU calls happen here, so its
// handlers are safe to run on the HTTP goroutine concurrently with the game
// loop, which syncs textures off a revision diff.
type module struct {
	store *store
	mux   *http.ServeMux
}

func newModule(st *store) *module {
	s := &module{store: st}

	// The control server mounts API under /api/game/ (prefix-stripped), so this
	// mux still sees the original "/api/cards" etc. paths — no route changes.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/cards", s.handleList)
	mux.HandleFunc("POST /api/cards", s.handleUpload)
	mux.HandleFunc("DELETE /api/cards/{id}", s.handleDelete)
	mux.HandleFunc("PATCH /api/cards/{id}", s.handlePatch)
	mux.HandleFunc("POST /api/cards/{id}/photos", s.handleAddPhoto)
	mux.HandleFunc("DELETE /api/cards/{id}/photos/{photoId}", s.handleDeletePhoto)
	mux.HandleFunc("GET /api/cards/{id}/photos/{photoId}/image", s.handleImage)
	mux.HandleFunc("POST /api/current", s.handleSetCurrent)
	mux.HandleFunc("POST /api/next", s.handleStep(+1))
	mux.HandleFunc("POST /api/prev", s.handleStep(-1))
	mux.HandleFunc("POST /api/photo/next", s.handlePhotoStep(+1))
	mux.HandleFunc("POST /api/photo/prev", s.handlePhotoStep(-1))
	s.mux = mux

	return s
}

// Page serves the deck-management SPA (mounted at /game/, prefix-stripped).
func (s *module) Page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

// API forwards the prefix-stripped request to the REST mux.
func (s *module) API(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// --- handlers ---

type photoDTO struct {
	ID string `json:"id"`
}

type cardDTO struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	IsCurrent bool       `json:"isCurrent"`
	Photos    []photoDTO `json:"photos"`
}

func toCardDTO(c Card, currentID string) cardDTO {
	photos := make([]photoDTO, 0, len(c.Photos))
	for _, p := range c.Photos {
		photos = append(photos, photoDTO{ID: p.ID})
	}
	return cardDTO{ID: c.ID, Name: c.Name, IsCurrent: c.ID == currentID, Photos: photos}
}

func (s *module) handleList(w http.ResponseWriter, r *http.Request) {
	cards, currentID, currentPhoto := s.store.list()
	out := struct {
		Cards        []cardDTO `json:"cards"`
		CurrentID    string    `json:"currentId"`
		CurrentPhoto int       `json:"currentPhoto"`
	}{Cards: make([]cardDTO, 0, len(cards)), CurrentID: currentID, CurrentPhoto: currentPhoto}
	for _, c := range cards {
		out.Cards = append(out.Cards, toCardDTO(c, currentID))
	}
	writeJSON(w, out)
}

func (s *module) handleUpload(w http.ResponseWriter, r *http.Request) {
	d, png, ok := s.readUploadImage(w, r)
	if !ok {
		return
	}
	name := cleanName(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	c, err := s.store.add(name, png, d)
	if err != nil {
		http.Error(w, "could not save card", http.StatusInternalServerError)
		return
	}
	writeJSON(w, toCardDTO(c, ""))
}

// handleAddPhoto appends another photo to an existing card.
func (s *module) handleAddPhoto(w http.ResponseWriter, r *http.Request) {
	d, png, ok := s.readUploadImage(w, r)
	if !ok {
		return
	}
	p, err := s.store.addPhoto(r.PathValue("id"), png, d)
	if err != nil {
		http.Error(w, "could not add photo (unknown card?)", http.StatusBadRequest)
		return
	}
	writeJSON(w, photoDTO{ID: p.ID})
}

// readUploadImage parses the multipart body and runs the "image" field through
// the decode/resize pipeline. On any problem it writes a 400 and returns ok
// false; the caller must stop. r.FormValue (used by callers for other fields)
// also triggers parsing, so it is safe to call before or after this.
func (s *module) readUploadImage(w http.ResponseWriter, r *http.Request) (*decoded, []byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "upload too large or malformed", http.StatusBadRequest)
		return nil, nil, false
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image file is required", http.StatusBadRequest)
		return nil, nil, false
	}
	defer file.Close()

	d, png, err := processUpload(file)
	if err != nil {
		http.Error(w, "could not read image (use PNG, JPEG, or GIF)", http.StatusBadRequest)
		return nil, nil, false
	}
	return d, png, true
}

func (s *module) handleDelete(w http.ResponseWriter, r *http.Request) {
	s.store.remove(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *module) handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	s.store.removePhoto(r.PathValue("id"), r.PathValue("photoId"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *module) handlePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Name  *string `json:"name"`
		Order *int    `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		name := cleanName(*body.Name)
		if name == "" {
			http.Error(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		s.store.rename(id, name)
	}
	if body.Order != nil {
		s.store.move(id, *body.Order)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *module) handleImage(w http.ResponseWriter, r *http.Request) {
	file, ok := s.store.fileForPhoto(r.PathValue("id"), r.PathValue("photoId"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(s.store.dir, file))
}

func (s *module) handleSetCurrent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Photo *int   `json:"photo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// With an explicit photo index, select that exact photo; otherwise select
	// the card and reset to its first photo.
	if body.Photo != nil {
		s.store.selectPhoto(body.ID, *body.Photo)
	} else {
		s.store.setCurrent(body.ID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *module) handleStep(delta int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.store.step(delta)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *module) handlePhotoStep(delta int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.store.stepPhoto(delta)
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}
	return name
}
