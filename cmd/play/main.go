package main

import (
	"fmt"
	"os"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/games"
	"github.com/anthonyrego/wand/games/colorsphere"
	"github.com/anthonyrego/wand/games/drumcircle"
	"github.com/anthonyrego/wand/games/flashcard"
	"github.com/anthonyrego/wand/games/flying"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/profile"
	"github.com/anthonyrego/wand/pkg/settings"
)

func main() {
	err := sdl.LoadLibrary(sdl.Path())
	if err != nil {
		fmt.Println("Loading embedded SDL3 library...")
		defer binsdl.Load().Unload()
	}

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_AUDIO); err != nil {
		panic("failed to initialize SDL: " + err.Error())
	}
	defer sdl.Quit()

	ds := settings.Default()

	// Personalization: load the saved owner name and serve a tiny web admin so a
	// parent can rename the wand ("EMMA'S WAND") from any device on the LAN.
	prof, err := profile.Load(profile.Dir())
	if err != nil {
		fmt.Println("Profile (using default name):", err)
	}
	psrv := profile.NewServer(prof, profile.Port())
	if err := psrv.Start(); err != nil {
		fmt.Println("Profile server:", err)
	} else {
		defer psrv.Stop()
		host := profile.LocalIP()
		if host == "" {
			host = "localhost"
		}
		fmt.Printf("Personalize the wand at http://%s:%d\n", host, profile.Port())
	}

	e, err := engine.New(profile.WindowTitle(prof.Name()), ds)
	if err != nil {
		panic(err)
	}
	defer e.Destroy()

	w := wand.New(9999)
	w.SetSmoothing(0.5)
	if err := w.Start(); err != nil {
		panic(err)
	}
	defer w.Stop()

	defs := []games.GameDef{
		{Name: "COLOR SPHERE", New: func(w *wand.Listener) engine.Game { return colorsphere.New(w) }},
		{Name: "FLYING", New: func(w *wand.Listener) engine.Game { return flying.New(w) }},
		{Name: "DRUM CIRCLE", New: func(w *wand.Listener) engine.Game { return drumcircle.New(w) }},
		{Name: "FLASHCARDS", New: func(w *wand.Listener) engine.Game { return flashcard.New(w) }},
	}

	app := games.NewApp(w, defs, prof)

	if err := e.Run(app); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
