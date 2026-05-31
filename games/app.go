package games

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/toybox/games/splash"
	"github.com/anthonyrego/toybox/pkg/control"
	"github.com/anthonyrego/toybox/pkg/engine"
	"github.com/anthonyrego/toybox/pkg/mesh"
	"github.com/anthonyrego/toybox/pkg/renderer"
	"github.com/anthonyrego/toybox/pkg/ui"
	"github.com/anthonyrego/toybox/wand"
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
	a.drawCalibratePrompt(e, cmdBuf, target)
	a.drawEscCounter(e, cmdBuf, target)
}

// drawCalibratePrompt centers a "hold the wand straight up, then press
// Calibrate" overlay (dim panel + upward arrow + two text lines) whenever the
// active game still needs calibration. The meshes are rebuilt and freed each
// frame so nothing leaks once the player calibrates. Drawn before the esc
// counter so that small top-left countdown stays on top.
func (a *App) drawCalibratePrompt(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	cp, ok := a.current.(CalibrationPrompter)
	if !ok || !cp.NeedsCalibration() {
		return
	}

	w := float32(e.Win.Width())
	h := float32(e.Win.Height())
	ortho := mgl32.Ortho2D(0, w, h, 0)

	// Dim full-screen panel for legibility (black, ~alpha 140).
	panel, err := quadMesh(e.Rend, 0, 0, w, h, 0, 0, 0, 140)
	if err != nil {
		return
	}
	defer panel.Destroy(e.Rend)

	// Upward-pointing arrow, sized relative to screen height, bright blue.
	const ar, ag, ab, aa uint8 = 120, 200, 255, 255
	headH := h * 0.10
	headW := h * 0.16
	shaftH := h * 0.14
	shaftW := h * 0.05
	cx := w / 2
	top := h * 0.18 // arrow apex near the upper third
	headBottom := top + headH
	arrowVerts := []renderer.Vertex{
		// head triangle (apex at top, base at headBottom)
		{X: cx, Y: top, Z: 0, R: ar, G: ag, B: ab, A: aa},
		{X: cx - headW/2, Y: headBottom, Z: 0, R: ar, G: ag, B: ab, A: aa},
		{X: cx + headW/2, Y: headBottom, Z: 0, R: ar, G: ag, B: ab, A: aa},
		// shaft rectangle (below the head)
		{X: cx - shaftW/2, Y: headBottom, Z: 0, R: ar, G: ag, B: ab, A: aa},
		{X: cx + shaftW/2, Y: headBottom, Z: 0, R: ar, G: ag, B: ab, A: aa},
		{X: cx + shaftW/2, Y: headBottom + shaftH, Z: 0, R: ar, G: ag, B: ab, A: aa},
		{X: cx - shaftW/2, Y: headBottom + shaftH, Z: 0, R: ar, G: ag, B: ab, A: aa},
	}
	arrowIdx := []uint16{0, 1, 2, 3, 4, 5, 3, 5, 6}
	arrow, err := newMesh(e.Rend, arrowVerts, arrowIdx)
	if err != nil {
		return
	}
	defer arrow.Destroy(e.Rend)

	// Two centered text lines below the arrow.
	const ps = float32(3)
	line1, w1, err := ui.NewTextMesh(e.Rend, "HOLD YOUR WAND POINTING STRAIGHT UP", ps, 255, 255, 255, 255)
	if err != nil {
		return
	}
	defer line1.Destroy(e.Rend)
	line2, w2, err := ui.NewTextMesh(e.Rend, "THEN PRESS CALIBRATE ON THE APP", ps, ar, ag, ab, 255)
	if err != nil {
		return
	}
	defer line2.Destroy(e.Rend)

	textY := headBottom + shaftH + h*0.10
	lineGap := h * 0.07

	pass := e.Rend.BeginUIPass(cmdBuf, target)
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: panel.VertexBuffer,
		IndexBuffer:  panel.IndexBuffer,
		IndexCount:   panel.IndexCount,
		Transform:    ortho,
	})
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: arrow.VertexBuffer,
		IndexBuffer:  arrow.IndexBuffer,
		IndexCount:   arrow.IndexCount,
		Transform:    ortho,
	})
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: line1.VertexBuffer,
		IndexBuffer:  line1.IndexBuffer,
		IndexCount:   line1.IndexCount,
		Transform:    ortho.Mul4(mgl32.Translate3D((w-w1)/2, textY, 0)),
	})
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: line2.VertexBuffer,
		IndexBuffer:  line2.IndexBuffer,
		IndexCount:   line2.IndexCount,
		Transform:    ortho.Mul4(mgl32.Translate3D((w-w2)/2, textY+lineGap, 0)),
	})
	e.Rend.EndUIPass(pass)
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

// newMesh uploads solid-color UI geometry to a GPU mesh, releasing the vertex
// buffer if the index buffer fails so nothing leaks on a partial upload.
func newMesh(r *renderer.Renderer, verts []renderer.Vertex, idx []uint16) (*mesh.Mesh, error) {
	vb, err := r.CreateVertexBuffer(verts)
	if err != nil {
		return nil, err
	}
	ib, err := r.CreateIndexBuffer(idx)
	if err != nil {
		r.ReleaseBuffer(vb)
		return nil, err
	}
	return &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(idx))}, nil
}

// quadMesh builds a solid-color axis-aligned rectangle (top-left origin) as a UI
// mesh, ready to draw under an Ortho2D transform.
func quadMesh(r *renderer.Renderer, x, y, w, h float32, cr, cg, cb, ca uint8) (*mesh.Mesh, error) {
	verts := []renderer.Vertex{
		{X: x, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y + h, Z: 0, R: cr, G: cg, B: cb, A: ca},
		{X: x, Y: y + h, Z: 0, R: cr, G: cg, B: cb, A: ca},
	}
	return newMesh(r, verts, []uint16{0, 1, 2, 0, 2, 3})
}
