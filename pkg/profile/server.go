package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Server exposes the owner name over HTTP so a parent can personalize the wand
// from any device on the LAN — the same manage-from-your-phone pattern the
// flashcard deck uses. It only ever touches the Store (CPU-side); there are no
// GPU calls here.
type Server struct {
	store *Store
	http  *http.Server
}

func NewServer(st *Store, port int) *Server {
	s := &Server{store: st}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /api/profile", s.handleGet)
	mux.HandleFunc("POST /api/profile", s.handleSet)

	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return s
}

// Start serves in a background goroutine, returning once the listener is bound
// so the caller knows the port is live (or sees the bind error).
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Println("profile server:", err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.http.Shutdown(ctx)
}

// Handler exposes the mux for tests.
func (s *Server) Handler() http.Handler { return s.http.Handler }

// --- handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

type profileDTO struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	name := s.store.Name()
	writeJSON(w, profileDTO{Name: name, Title: Title(name)})
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := s.store.SetName(body.Name)
	writeJSON(w, profileDTO{Name: name, Title: Title(name)})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// LocalIP returns the host's primary LAN IPv4 address (best effort) by checking
// which local interface would reach the internet. No packets are sent. Returns
// "" if it can't be determined.
func LocalIP() string {
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
