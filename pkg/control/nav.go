package control

import "sync"

// SplashID is the reserved "game id" for the boot/idle splash screen (no game
// running). The web posts it to return the TV to the splash.
const SplashID = "splash"

// GameInfo is the wire description of a registered game. The control server
// learns the game list as data (built in cmd/play from the registry) so it
// never has to import the games package.
type GameInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RequiresWand bool   `json:"requiresWand"` // motion-controlled — web shows a wand icon
}

// NavStore is the game-switch channel between the web (which requests a switch)
// and the game loop (which performs it). All access is guarded by mu.
type NavStore struct {
	mu        sync.Mutex
	games     []GameInfo // immutable after construction
	requested string     // "" = no pending request
	current   string     // SplashID at boot; set by the App after a switch completes
}

// NewNavStore builds a NavStore over the given game list, starting on the splash.
func NewNavStore(games []GameInfo) *NavStore {
	return &NavStore{games: games, current: SplashID}
}

// Games returns a copy of the registered game list (for GET /api/games).
func (n *NavStore) Games() []GameInfo {
	out := make([]GameInfo, len(n.games))
	copy(out, n.games)
	return out
}

func (n *NavStore) valid(id string) bool {
	if id == SplashID {
		return true
	}
	for _, g := range n.games {
		if g.ID == id {
			return true
		}
	}
	return false
}

// Request records a pending switch to id. It returns false (and records
// nothing) if id is neither a known game nor the splash sentinel.
func (n *NavStore) Request(id string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.valid(id) {
		return false
	}
	n.requested = id
	return true
}

// Current returns the id of the on-screen game (or SplashID).
func (n *NavStore) Current() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.current
}

// TakeRequest returns and clears a pending switch request. The App calls this
// once per frame; ok is false when nothing is pending.
func (n *NavStore) TakeRequest() (string, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.requested == "" {
		return "", false
	}
	id := n.requested
	n.requested = ""
	return id, true
}

// SetCurrent records the on-screen game after the App completes a switch.
func (n *NavStore) SetCurrent(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.current = id
}
