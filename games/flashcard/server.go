package flashcard

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxUploadBytes = 15 << 20 // 15 MiB ceiling on a single upload
	maxNameLen     = 40
)

// server exposes the deck over HTTP for management from any device on the LAN.
// It only ever touches the store (CPU-side); no GPU calls happen here.
type server struct {
	store *store
	http  *http.Server
}

func newServer(st *store, port int) *server {
	s := &server{store: st}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /api/cards", s.handleList)
	mux.HandleFunc("POST /api/cards", s.handleUpload)
	mux.HandleFunc("DELETE /api/cards/{id}", s.handleDelete)
	mux.HandleFunc("PATCH /api/cards/{id}", s.handlePatch)
	mux.HandleFunc("GET /api/cards/{id}/image", s.handleImage)
	mux.HandleFunc("POST /api/current", s.handleSetCurrent)
	mux.HandleFunc("POST /api/next", s.handleStep(+1))
	mux.HandleFunc("POST /api/prev", s.handleStep(-1))

	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return s
}

// start serves in a background goroutine. Returns once the listener is bound so
// the caller can be sure the port is live (or report the bind error).
func (s *server) start() error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Println("flashcard server:", err)
		}
	}()
	return nil
}

func (s *server) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
}

// --- handlers ---

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

type cardDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsCurrent bool   `json:"isCurrent"`
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	cards, currentID := s.store.list()
	out := struct {
		Cards     []cardDTO `json:"cards"`
		CurrentID string    `json:"currentId"`
	}{Cards: make([]cardDTO, 0, len(cards)), CurrentID: currentID}
	for _, c := range cards {
		out.Cards = append(out.Cards, cardDTO{ID: c.ID, Name: c.Name, IsCurrent: c.ID == currentID})
	}
	writeJSON(w, out)
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "upload too large or malformed", http.StatusBadRequest)
		return
	}

	name := cleanName(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	d, png, err := processUpload(file)
	if err != nil {
		http.Error(w, "could not read image (use PNG, JPEG, or GIF)", http.StatusBadRequest)
		return
	}

	c, err := s.store.add(name, png, d)
	if err != nil {
		http.Error(w, "could not save card", http.StatusInternalServerError)
		return
	}
	writeJSON(w, cardDTO{ID: c.ID, Name: c.Name})
}

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	s.store.remove(r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handlePatch(w http.ResponseWriter, r *http.Request) {
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

func (s *server) handleImage(w http.ResponseWriter, r *http.Request) {
	file, ok := s.store.fileForID(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(s.store.dir, file))
}

func (s *server) handleSetCurrent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.store.setCurrent(body.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleStep(delta int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.store.step(delta)
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

// localIP returns the host's primary LAN IPv4 address (best effort) by checking
// which local interface would be used to reach the internet. No packets are
// sent. Returns "" if it can't be determined.
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}
