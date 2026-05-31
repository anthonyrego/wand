package games

import (
	"github.com/anthonyrego/toybox/games/splash"
	"github.com/anthonyrego/toybox/pkg/control"
	"github.com/anthonyrego/toybox/pkg/engine"
	"github.com/anthonyrego/toybox/wand"
)

// GameDef describes a registered game. ID is a stable, author-assigned slug
// used on the wire (web → switch); Name is the display name.
type GameDef struct {
	ID   string
	Name string
	New  func(w *wand.Listener) engine.Game
}

// WebProvider is optionally implemented by a game that contributes a web page +
// REST to the control server — its own settings, editable while it runs. Games
// without one have no per-game page (the App installs nil on switch).
type WebProvider interface {
	WebModule() control.GameModule
}

// CalibrationPrompter is implemented by wand games that need an on-screen
// "hold the wand straight up, then press Calibrate" prompt until the player
// calibrates. The App draws the prompt centrally while NeedsCalibration is true.
type CalibrationPrompter interface {
	NeedsCalibration() bool
}

// ControlBridge wires the App to the always-on control server and its shared
// stores. The App polls Nav/Video each frame and swaps the active web module on
// the server as games start and stop; Title/URL parameterize the splash.
type ControlBridge struct {
	Server *control.Server
	Nav    *control.NavStore
	Video  *control.VideoStore
	Title  splash.TitleProvider
	URL    string
}
