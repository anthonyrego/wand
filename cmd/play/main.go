package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/anthonyrego/toybox/wand"
	"github.com/anthonyrego/toybox/games"
	"github.com/anthonyrego/toybox/games/drumcircle"
	"github.com/anthonyrego/toybox/games/flashcard"
	"github.com/anthonyrego/toybox/games/flying"
	"github.com/anthonyrego/toybox/pkg/control"
	"github.com/anthonyrego/toybox/pkg/control/profile"
	"github.com/anthonyrego/toybox/pkg/engine"
)

// version is stamped at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fmt.Println("Toy Box", version)

	err := sdl.LoadLibrary(sdl.Path())
	if err != nil {
		fmt.Println("Loading embedded SDL3 library...")
		defer binsdl.Load().Unload()
	}

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_AUDIO); err != nil {
		panic("failed to initialize SDL: " + err.Error())
	}
	defer sdl.Quit()

	// Personalization + display settings, both persisted under ~/.toybox.
	prof, err := profile.Load(profile.Dir())
	if err != nil {
		fmt.Println("Profile (using default name):", err)
	}
	videoStore := control.NewVideoStore(filepath.Join(profile.Dir(), "settings.json"))

	// Game registry. IDs are stable wire slugs (used by the web); Names show on
	// screen. The control server learns the list as data, so it never imports
	// the games package.
	defs := []games.GameDef{
		{ID: "flying", Name: "FLYING", New: func(w *wand.Listener) engine.Game { return flying.New(w) }},
		{ID: "drum-circle", Name: "DRUM CIRCLE", New: func(w *wand.Listener) engine.Game { return drumcircle.New(w) }},
		{ID: "flashcards", Name: "FLASHCARDS", New: func(w *wand.Listener) engine.Game { return flashcard.New(w) }},
	}
	infos := make([]control.GameInfo, len(defs))
	for i, d := range defs {
		infos[i] = control.GameInfo{ID: d.ID, Name: d.Name}
	}
	navStore := control.NewNavStore(infos)

	e, err := engine.New(profile.WindowTitle(prof.Name()), videoStore.Settings())
	if err != nil {
		panic(err)
	}
	defer e.Destroy()

	// Snapshot the display's selectable resolutions once on the main thread, so
	// the web UI can offer a dropdown without any HTTP-side SDL access.
	var modes []control.Resolution
	for _, m := range e.Win.DisplayModes() {
		modes = append(modes, control.Resolution{W: m.W, H: m.H})
	}

	// One always-on control server on the canonical port (8080). The screen is a
	// pure display; a parent drives everything from a phone or laptop.
	port := control.Port()
	srv := control.NewServer(control.Deps{Name: prof, Video: videoStore, Nav: navStore, Resolutions: modes}, port)
	if err := srv.Start(); err != nil {
		panic("control server: " + err.Error())
	}
	defer srv.Stop()
	url := control.URL(port)
	fmt.Println("Control the wand at", url)

	w := wand.New(9999)
	w.SetSmoothing(0.5)
	if err := w.Start(); err != nil {
		panic(err)
	}
	defer w.Stop()

	app := games.NewApp(w, defs, games.ControlBridge{
		Server: srv,
		Nav:    navStore,
		Video:  videoStore,
		Title:  prof,
		URL:    url,
	})

	if err := e.Run(app); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
