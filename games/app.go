package games

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/games/splash"
	"github.com/anthonyrego/wand/pkg/control"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/ui"
)

const (
	escQuitPresses = 3   // consecutive Esc presses that quit the app
	escResetSecs   = 1.5 // idle gap (s) that resets the Esc counter
)

// App is the top-level engine.Game. It owns no input of its own beyond the
// Esc-to-quit chord; all control (which game runs, settings) arrives over the
// web via the control server's shared stores, which App polls each frame.
type App struct {
	wand    *wand.Listener
	defs    []GameDef
	defByID map[string]GameDef
	bridge  ControlBridge

	current   engine.Game
	currentID string

	seenVideoRev uint64

	escCount int
	escTimer float32
	escMesh  *mesh.Mesh
}

func NewApp(w *wand.Listener, defs []GameDef, bridge ControlBridge) *App {
	byID := make(map[string]GameDef, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
	}
	return &App{
		wand:      w,
		defs:      defs,
		defByID:   byID,
		bridge:    bridge,
		currentID: control.SplashID,
	}
}

func (a *App) Init(e *engine.Engine) error {
	a.current = splash.New(a.bridge.Title, a.bridge.URL)
	a.currentID = control.SplashID
	a.bridge.Server.SetActiveModule(nil)
	a.bridge.Nav.SetCurrent(control.SplashID)
	// Prime the video cursor so the saved settings (already applied at boot)
	// don't trigger a redundant re-apply on the first frame.
	a.seenVideoRev = a.bridge.Video.Revision()
	return a.current.Init(e)
}

func (a *App) Update(e *engine.Engine, dt float32) bool {
	if a.checkQuit(e, dt) {
		return false
	}

	// Web-requested game switch (consumed once).
	if id, ok := a.bridge.Nav.TakeRequest(); ok && id != a.currentID {
		if err := a.switchTo(e, id); err != nil {
			fmt.Println("game switch:", err)
			return false
		}
	}

	// Web-requested display settings, applied on save.
	a.applyVideoIfChanged(e)

	return a.current.Update(e, dt)
}

// switchTo destroys the current game and starts the one named by id (or the
// splash for control.SplashID). The active web module is withdrawn before the
// outgoing game is destroyed and installed after the new one inits — see
// control.Server.SetActiveModule for the ordering/safety contract.
func (a *App) switchTo(e *engine.Engine, id string) error {
	a.bridge.Server.SetActiveModule(nil)
	a.current.Destroy(e)

	if def, ok := a.defByID[id]; ok {
		a.current = def.New(a.wand)
	} else {
		a.current = splash.New(a.bridge.Title, a.bridge.URL)
		id = control.SplashID
	}
	a.currentID = id

	if err := a.current.Init(e); err != nil {
		return err
	}

	if wp, ok := a.current.(WebProvider); ok {
		a.bridge.Server.SetActiveModule(wp.WebModule())
	} else {
		a.bridge.Server.SetActiveModule(nil)
	}
	a.bridge.Nav.SetCurrent(id)
	return nil
}

func (a *App) applyVideoIfChanged(e *engine.Engine) {
	s, rev, ok := a.bridge.Video.SettingsIfChanged(a.seenVideoRev)
	if !ok {
		return
	}
	a.seenVideoRev = rev
	e.ApplyDisplaySettings(s.Fullscreen, s.WindowWidth, s.WindowHeight, s.RenderDistance)
}

// checkQuit implements the only keyboard interaction left: three Esc presses
// (within escResetSecs of each other) quit the app. A gap resets the count.
func (a *App) checkQuit(e *engine.Engine, dt float32) bool {
	if e.Input.IsKeyPressed(sdl.K_ESCAPE) {
		a.escCount++
		a.escTimer = 0
		if a.escCount >= escQuitPresses {
			return true
		}
	} else {
		a.escTimer += dt
		if a.escTimer > escResetSecs {
			a.escCount = 0
		}
	}
	return false
}

func (a *App) Render(e *engine.Engine, frame renderer.RenderFrame) {
	a.current.Render(e, frame)
}

func (a *App) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	a.current.Overlay(e, cmdBuf, target)
	a.drawEscCounter(e, cmdBuf, target)
}

// drawEscCounter overlays the quit countdown on top of whatever is on screen
// (a second LOAD UI pass composites over the game's own overlay).
func (a *App) drawEscCounter(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	if a.escCount <= 0 {
		if a.escMesh != nil {
			a.escMesh.Destroy(e.Rend)
			a.escMesh = nil
		}
		return
	}

	if a.escMesh != nil {
		a.escMesh.Destroy(e.Rend)
		a.escMesh = nil
	}
	msg := fmt.Sprintf("PRESS ESC %d MORE TIME(S) TO QUIT", escQuitPresses-a.escCount)
	const ps = float32(2) // small, unobtrusive
	m, _, err := ui.NewTextMesh(e.Rend, msg, ps, 240, 200, 200, 255)
	if err != nil {
		return
	}
	a.escMesh = m

	w := float32(e.Win.Width())
	h := float32(e.Win.Height())
	ortho := mgl32.Ortho2D(0, w, h, 0)
	pass := e.Rend.BeginUIPass(cmdBuf, target)
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: m.VertexBuffer,
		IndexBuffer:  m.IndexBuffer,
		IndexCount:   m.IndexCount,
		Transform:    ortho.Mul4(mgl32.Translate3D(12, 12, 0)), // top-left
	})
	e.Rend.EndUIPass(pass)
}

func (a *App) Destroy(e *engine.Engine) {
	// Stop routing web requests to the game before tearing it down.
	a.bridge.Server.SetActiveModule(nil)
	if a.escMesh != nil {
		a.escMesh.Destroy(e.Rend)
		a.escMesh = nil
	}
	a.current.Destroy(e)
}
