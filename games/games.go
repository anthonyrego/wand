package games

import (
	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/pkg/engine"
)

// GameDef describes a game available in the selector.
type GameDef struct {
	Name string
	New  func(w *wand.Listener) engine.Game
}

// GameSwitcher is optionally implemented by games that support
// returning to the game selector via the pause menu.
type GameSwitcher interface {
	WantsChangeGame() bool
}
