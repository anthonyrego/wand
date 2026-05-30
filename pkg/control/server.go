// Package control is the wand's single always-on web server. It runs for the
// app's whole life on one LAN port (8080) and is the only control surface: a
// parent drives everything — picking the game, naming the wand, video settings,
// and each game's own tunables — from a phone or laptop. The screen is a pure
// display.
//
// All web handlers touch only mutex-guarded, CPU-side stores (name/video/nav)
// and the active game's module; they never call SDL, the GPU, or the engine.
// The game loop (games.App) polls those stores each frame and performs the
// actual switches, ApplyDisplaySettings, and GPU work on its own thread. This
// mirrors the revision-diff pattern the flashcard deck already used.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anthonyrego/toybox/pkg/control/profile"
	"github.com/anthonyrego/toybox/pkg/settings"
)

// Deps are the shared, thread-safe stores the control server reads and writes,
// plus the static list of selectable display resolutions.
type Deps struct {
	Name        *profile.Store
	Video       *VideoStore
	Nav         *NavStore
	Resolutions []Resolution
}

// Server is the always-on control server. The active game's GameModule is
// swapped in/out by the App as games start and stop.
type Server struct {
	deps Deps
	http *http.Server

	activeMu sync.RWMutex
	active   GameModule // nil when on the splash / no game running
}

// NewServer builds the control server (not yet listening — call Start).
func NewServer(deps Deps, port int) *Server {
	s := &Server{deps: deps}

	mux := http.NewServeMux()
	// Registered without a method so the path-specific "/game/" and "/api/game/"
	// subtrees are unambiguously more specific (Go 1.22 mux flags a method-vs-path
	// specificity tie as a conflict). handleIndex 404s anything but "/".
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("GET /api/profile", s.handleProfileGet)
	mux.HandleFunc("POST /api/profile", s.handleProfileSet)
	mux.HandleFunc("GET /api/games", s.handleGames)
	mux.HandleFunc("GET /api/current", s.handleCurrentGet)
	mux.HandleFunc("POST /api/current", s.handleCurrentSet)
	mux.HandleFunc("GET /api/video", s.handleVideoGet)
	mux.HandleFunc("POST /api/video", s.handleVideoSet)
	mux.HandleFunc("GET /api/video/modes", s.handleVideoModes)
	// The active game's page and REST. StripPrefix means a module sees "/...".
	mux.Handle("/game/", http.StripPrefix("/game", http.HandlerFunc(s.dispatchPage)))
	mux.Handle("/api/game/", http.StripPrefix("/api/game", http.HandlerFunc(s.dispatchAPI)))

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
			fmt.Println("control server:", err)
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

// SetActiveModule swaps the web module for the active game. The App calls this
// on the game-loop thread during a switch, in this order:
//
//	SetActiveModule(nil)   // stop routing new requests to the outgoing module
//	oldGame.Destroy(e)     // free its resources / close its store
//	newGame.Init(e)
//	SetActiveModule(newGame.WebModule())
//
// The Lock here cannot complete until every in-flight reader (a dispatch holding
// the RLock for the whole module call) has returned, so no HTTP goroutine is
// executing inside the outgoing module when it is destroyed. Pass nil for games
// with no web page (and for the splash).
func (s *Server) SetActiveModule(m GameModule) {
	s.activeMu.Lock()
	s.active = m
	s.activeMu.Unlock()
}

// dispatchPage/dispatchAPI hold the RLock for the FULL duration of the module
// call. This is the barrier that makes SetActiveModule(nil) safe (see above):
// the swap waits for in-flight game requests to finish. Reads are concurrent,
// so normal traffic is never serialized — only a switch waits on active reads.
func (s *Server) dispatchPage(w http.ResponseWriter, r *http.Request) {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	if s.active == nil {
		http.Error(w, "no game running", http.StatusServiceUnavailable)
		return
	}
	s.active.Page(w, r)
}

func (s *Server) dispatchAPI(w http.ResponseWriter, r *http.Request) {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	if s.active == nil {
		http.Error(w, "no game running", http.StatusServiceUnavailable)
		return
	}
	s.active.API(w, r)
}

// --- app-level handlers ---

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

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	name := s.deps.Name.Name()
	writeJSON(w, profileDTO{Name: name, Title: profile.Title(name)})
}

func (s *Server) handleProfileSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	name := s.deps.Name.SetName(body.Name)
	writeJSON(w, profileDTO{Name: name, Title: profile.Title(name)})
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.deps.Nav.Games())
}

type currentDTO struct {
	Current string `json:"current"`
	HasPage bool   `json:"hasPage"` // the active game has a web page (settings/admin)
}

func (s *Server) handleCurrentGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, currentDTO{Current: s.deps.Nav.Current(), HasPage: s.hasActiveModule()})
}

func (s *Server) hasActiveModule() bool {
	s.activeMu.RLock()
	defer s.activeMu.RUnlock()
	return s.active != nil
}

func (s *Server) handleCurrentSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !s.deps.Nav.Request(body.ID) {
		http.Error(w, "unknown game", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleVideoGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.deps.Video.Settings())
}

func (s *Server) handleVideoSet(w http.ResponseWriter, r *http.Request) {
	var in settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.deps.Video.Set(in))
}

func (s *Server) handleVideoModes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.deps.Resolutions)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
