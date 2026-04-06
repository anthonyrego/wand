package ui

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand/pkg/input"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/sign"
	"github.com/anthonyrego/wand/pkg/window"
)

type MenuState int

const (
	Hidden MenuState = iota
	Main
	Settings
)

type Action int

const (
	ActionNone Action = iota
	ActionQuit
	ActionApplySettings
	ActionChangeGame
)

// textEntry holds white (selected) and gray (unselected) variants of a text mesh.
type textEntry struct {
	white *mesh.Mesh
	gray  *mesh.Mesh
	dim   *mesh.Mesh // dimmer variant for inactive elements
	width float32
}

func (t *textEntry) meshFor(selected bool) *mesh.Mesh {
	if selected {
		return t.white
	}
	return t.gray
}

func (t *textEntry) destroy(r *renderer.Renderer) {
	if t.white != nil {
		t.white.Destroy(r)
	}
	if t.gray != nil {
		t.gray.Destroy(r)
	}
	if t.dim != nil {
		t.dim.Destroy(r)
	}
}

type PauseMenu struct {
	state    MenuState
	selIndex int
	ps       float32 // screen pixels per font pixel

	overlay *mesh.Mesh

	title textEntry // "PAUSED" (white only)
	arrow textEntry // ">" (white only)

	// Main menu: resume(0), settings(1), change game(2), quit(3)
	mainItems [4]textEntry

	// Settings
	settingsTitle textEntry   // "SETTINGS" (white only)
	fsLabel       textEntry   // "FULLSCREEN"
	fsOn          textEntry   // "ON"
	fsOff         textEntry   // "OFF"
	resLabel      textEntry   // "RESOLUTION"
	resOpts       []textEntry // dynamic from SDL3
	rdLabel       textEntry   // "DRAW DISTANCE"
	rdOpts        []textEntry // predefined list
	apply         textEntry   // "APPLY"
	back          textEntry   // "BACK"

	resolutions []window.Resolution

	// Applied = what the window currently is
	appliedFS     bool
	appliedResIdx int
	appliedRD     int // index into RenderDistances

	// Pending = what user has toggled but not yet applied
	pendingFS     bool
	pendingResIdx int
	pendingRD     int
	dirty         bool
}

func newEntry(r *renderer.Renderer, text string, ps float32) (textEntry, error) {
	w, width, err := NewTextMesh(r, text, ps, 255, 255, 255, 255)
	if err != nil {
		return textEntry{}, err
	}
	g, _, err := NewTextMesh(r, text, ps, 120, 120, 120, 255)
	if err != nil {
		w.Destroy(r)
		return textEntry{}, err
	}
	return textEntry{white: w, gray: g, width: width}, nil
}

func newDimEntry(r *renderer.Renderer, text string, ps float32) (textEntry, error) {
	w, width, err := NewTextMesh(r, text, ps, 255, 255, 255, 255)
	if err != nil {
		return textEntry{}, err
	}
	g, _, err := NewTextMesh(r, text, ps, 120, 120, 120, 255)
	if err != nil {
		w.Destroy(r)
		return textEntry{}, err
	}
	d, _, err := NewTextMesh(r, text, ps, 60, 60, 60, 255)
	if err != nil {
		w.Destroy(r)
		g.Destroy(r)
		return textEntry{}, err
	}
	return textEntry{white: w, gray: g, dim: d, width: width}, nil
}

func newWhiteOnly(r *renderer.Renderer, text string, ps float32) (textEntry, error) {
	w, width, err := NewTextMesh(r, text, ps, 255, 255, 255, 255)
	if err != nil {
		return textEntry{}, err
	}
	return textEntry{white: w, width: width}, nil
}

var RenderDistances = []int{500, 750, 1000, 1250, 1500, 2000}

const FontScale = 4

func NewPauseMenu(r *renderer.Renderer, resolutions []window.Resolution) *PauseMenu {
	ps := float32(FontScale)
	p := &PauseMenu{ps: ps, resolutions: resolutions}

	// Overlay: unit quad with semi-transparent black
	overlayVerts := []renderer.Vertex{
		{X: 0, Y: 0, Z: 0, R: 0, G: 0, B: 0, A: 160},
		{X: 1, Y: 0, Z: 0, R: 0, G: 0, B: 0, A: 160},
		{X: 1, Y: 1, Z: 0, R: 0, G: 0, B: 0, A: 160},
		{X: 0, Y: 1, Z: 0, R: 0, G: 0, B: 0, A: 160},
	}
	overlayIdx := []uint16{0, 1, 2, 0, 2, 3}
	vb, err := r.CreateVertexBuffer(overlayVerts)
	if err != nil {
		fmt.Println("Warning: UI overlay vertex buffer:", err)
		return p
	}
	ib, err := r.CreateIndexBuffer(overlayIdx)
	if err != nil {
		r.ReleaseBuffer(vb)
		fmt.Println("Warning: UI overlay index buffer:", err)
		return p
	}
	p.overlay = &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: 6}

	// Build text meshes (errors are non-fatal, menu just won't render that item)
	p.title, _ = newWhiteOnly(r, "PAUSED", ps)
	p.arrow, _ = newWhiteOnly(r, ">", ps)

	p.mainItems[0], _ = newEntry(r, "RESUME", ps)
	p.mainItems[1], _ = newEntry(r, "SETTINGS", ps)
	p.mainItems[2], _ = newEntry(r, "CHANGE GAME", ps)
	p.mainItems[3], _ = newEntry(r, "QUIT", ps)

	p.settingsTitle, _ = newWhiteOnly(r, "SETTINGS", ps)
	p.fsLabel, _ = newEntry(r, "FULLSCREEN", ps)
	p.fsOn, _ = newEntry(r, "ON", ps)
	p.fsOff, _ = newEntry(r, "OFF", ps)
	p.resLabel, _ = newEntry(r, "RESOLUTION", ps)
	p.apply, _ = newDimEntry(r, "APPLY", ps)
	p.back, _ = newEntry(r, "BACK", ps)

	for _, res := range resolutions {
		label := fmt.Sprintf("%dX%d", res.W, res.H)
		entry, _ := newEntry(r, label, ps)
		p.resOpts = append(p.resOpts, entry)
	}

	p.rdLabel, _ = newEntry(r, "DRAW DISTANCE", ps)
	for _, v := range RenderDistances {
		entry, _ := newEntry(r, fmt.Sprintf("%d", v), ps)
		p.rdOpts = append(p.rdOpts, entry)
	}

	return p
}

func (p *PauseMenu) IsActive() bool {
	return p.state != Hidden
}

// SetAppliedState sets the current applied state (call at init).
func (p *PauseMenu) SetAppliedState(fullscreen bool, resIndex, rdIndex int) {
	p.appliedFS = fullscreen
	p.appliedResIdx = resIndex
	p.appliedRD = rdIndex
	p.pendingFS = fullscreen
	p.pendingResIdx = resIndex
	p.pendingRD = rdIndex
	p.dirty = false
}

// PendingFullscreen returns the pending fullscreen state.
func (p *PauseMenu) PendingFullscreen() bool {
	return p.pendingFS
}

// PendingResolution returns the pending resolution width and height.
func (p *PauseMenu) PendingResolution() (w, h int) {
	if p.pendingResIdx < 0 || p.pendingResIdx >= len(p.resolutions) {
		return 1280, 720
	}
	r := p.resolutions[p.pendingResIdx]
	return r.W, r.H
}

// PendingRenderDistance returns the pending render distance value.
func (p *PauseMenu) PendingRenderDistance() float32 {
	if p.pendingRD < 0 || p.pendingRD >= len(RenderDistances) {
		return 1500
	}
	return float32(RenderDistances[p.pendingRD])
}

// ConfirmApply copies pending to applied, clears dirty.
func (p *PauseMenu) ConfirmApply() {
	p.appliedFS = p.pendingFS
	p.appliedResIdx = p.pendingResIdx
	p.appliedRD = p.pendingRD
	p.dirty = false
}

func (p *PauseMenu) updateDirty() {
	p.dirty = p.pendingFS != p.appliedFS || p.pendingResIdx != p.appliedResIdx ||
		p.pendingRD != p.appliedRD
}

func (p *PauseMenu) HandleInput(inp *input.Input) Action {
	if inp.IsKeyPressed(sdl.K_ESCAPE) {
		if p.state == Hidden {
			p.state = Main
			p.selIndex = 0
			return ActionNone
		}
		if p.state == Settings {
			// Reset pending on back to main
			p.pendingFS = p.appliedFS
			p.pendingResIdx = p.appliedResIdx
			p.pendingRD = p.appliedRD
			p.dirty = false
			p.state = Main
			p.selIndex = 1
			return ActionNone
		}
		// Main menu — close
		p.state = Hidden
		return ActionNone
	}

	if p.state == Hidden {
		return ActionNone
	}

	if p.state == Main {
		if inp.IsKeyPressed(sdl.K_UP) {
			p.selIndex--
			if p.selIndex < 0 {
				p.selIndex = 3
			}
		}
		if inp.IsKeyPressed(sdl.K_DOWN) {
			p.selIndex++
			if p.selIndex > 3 {
				p.selIndex = 0
			}
		}
		if inp.IsKeyPressed(sdl.K_RETURN) {
			switch p.selIndex {
			case 0: // Resume
				p.state = Hidden
			case 1: // Settings
				p.state = Settings
				p.selIndex = 4 // Start on BACK
				// Copy applied to pending on enter
				p.pendingFS = p.appliedFS
				p.pendingResIdx = p.appliedResIdx
				p.pendingRD = p.appliedRD
				p.dirty = false
			case 2: // Change Game
				p.state = Hidden
				return ActionChangeGame
			case 3: // Quit
				return ActionQuit
			}
		}
		return ActionNone
	}

	// Settings (5 items: 0=fullscreen, 1=resolution, 2=draw distance, 3=apply, 4=back)
	if inp.IsKeyPressed(sdl.K_UP) {
		p.selIndex--
		if p.selIndex < 0 {
			p.selIndex = 4
		}
	}
	if inp.IsKeyPressed(sdl.K_DOWN) {
		p.selIndex++
		if p.selIndex > 4 {
			p.selIndex = 0
		}
	}
	// Left/Right toggles fullscreen on item 0
	if p.selIndex == 0 && (inp.IsKeyPressed(sdl.K_LEFT) || inp.IsKeyPressed(sdl.K_RIGHT) || inp.IsKeyPressed(sdl.K_RETURN)) {
		p.pendingFS = !p.pendingFS
		p.updateDirty()
	}
	if inp.IsKeyPressed(sdl.K_RETURN) {
		switch p.selIndex {
		case 3: // Apply
			if p.dirty {
				return ActionApplySettings
			}
		case 4: // Back
			p.pendingFS = p.appliedFS
			p.pendingResIdx = p.appliedResIdx
			p.pendingRD = p.appliedRD
			p.dirty = false
			p.state = Main
			p.selIndex = 1
		}
	}
	// Left/Right for resolution cycling
	if p.selIndex == 1 && len(p.resolutions) > 0 {
		if inp.IsKeyPressed(sdl.K_LEFT) {
			p.pendingResIdx--
			if p.pendingResIdx < 0 {
				p.pendingResIdx = len(p.resolutions) - 1
			}
			p.updateDirty()
		}
		if inp.IsKeyPressed(sdl.K_RIGHT) {
			p.pendingResIdx++
			if p.pendingResIdx >= len(p.resolutions) {
				p.pendingResIdx = 0
			}
			p.updateDirty()
		}
	}
	// Left/Right for render distance cycling
	if p.selIndex == 2 && len(RenderDistances) > 0 {
		if inp.IsKeyPressed(sdl.K_LEFT) {
			p.pendingRD--
			if p.pendingRD < 0 {
				p.pendingRD = len(RenderDistances) - 1
			}
			p.updateDirty()
		}
		if inp.IsKeyPressed(sdl.K_RIGHT) {
			p.pendingRD++
			if p.pendingRD >= len(RenderDistances) {
				p.pendingRD = 0
			}
			p.updateDirty()
		}
	}

	return ActionNone
}

func (p *PauseMenu) Render(r *renderer.Renderer, cmdBuf *sdl.GPUCommandBuffer, swapchainTex *sdl.GPUTexture, screenW, screenH int) {
	if p.state == Hidden || p.overlay == nil {
		return
	}

	ortho := mgl32.Ortho2D(0, float32(screenW), float32(screenH), 0)
	pass := r.BeginUIPass(cmdBuf, swapchainTex)

	draw := func(m *mesh.Mesh, transform mgl32.Mat4) {
		if m == nil {
			return
		}
		r.DrawUI(cmdBuf, pass, renderer.DrawCall{
			VertexBuffer: m.VertexBuffer,
			IndexBuffer:  m.IndexBuffer,
			IndexCount:   m.IndexCount,
			Transform:    transform,
		})
	}

	// Dark overlay
	draw(p.overlay, ortho.Mul4(mgl32.Scale3D(float32(screenW), float32(screenH), 1)))

	sw := float32(screenW)
	sh := float32(screenH)
	charH := p.ps * float32(sign.CharHeight)
	lineH := charH + p.ps*3

	at := func(x, y float32) mgl32.Mat4 {
		return ortho.Mul4(mgl32.Translate3D(x, y, 0))
	}

	drawArrow := func(x, y float32) {
		draw(p.arrow.white, at(x-p.arrow.width-p.ps*2, y))
	}

	if p.state == Main {
		// Title
		titleY := sh*0.30 - charH/2
		draw(p.title.white, at((sw-p.title.width)/2, titleY))

		// Menu items
		startY := sh * 0.45
		for i := range p.mainItems {
			y := startY + float32(i)*lineH
			sel := i == p.selIndex
			x := (sw - p.mainItems[i].width) / 2
			draw(p.mainItems[i].meshFor(sel), at(x, y))
			if sel {
				drawArrow(x, y)
			}
		}
	} else {
		// Settings title
		titleY := sh*0.30 - charH/2
		draw(p.settingsTitle.white, at((sw-p.settingsTitle.width)/2, titleY))

		startY := sh * 0.45
		gap := p.ps * 3

		// Item 0: Fullscreen
		{
			i := 0
			y := startY + float32(i)*lineH
			sel := p.selIndex == i
			val := &p.fsOff
			if p.pendingFS {
				val = &p.fsOn
			}
			totalW := p.fsLabel.width + gap + val.width
			lx := (sw - totalW) / 2
			draw(p.fsLabel.meshFor(sel), at(lx, y))
			draw(val.meshFor(sel), at(lx+p.fsLabel.width+gap, y))
			if sel {
				drawArrow(lx, y)
			}
		}

		// Item 1: Resolution
		{
			i := 1
			y := startY + float32(i)*lineH
			sel := p.selIndex == i
			resIdx := p.pendingResIdx
			if resIdx < 0 || resIdx >= len(p.resOpts) {
				resIdx = 0
			}
			val := &p.resOpts[resIdx]
			totalW := p.resLabel.width + gap + val.width
			lx := (sw - totalW) / 2
			draw(p.resLabel.meshFor(sel), at(lx, y))
			draw(val.meshFor(sel), at(lx+p.resLabel.width+gap, y))
			if sel {
				drawArrow(lx, y)
			}
		}

		// Item 2: Draw Distance
		{
			i := 2
			y := startY + float32(i)*lineH
			sel := p.selIndex == i
			rdIdx := p.pendingRD
			if rdIdx < 0 || rdIdx >= len(p.rdOpts) {
				rdIdx = 0
			}
			val := &p.rdOpts[rdIdx]
			totalW := p.rdLabel.width + gap + val.width
			lx := (sw - totalW) / 2
			draw(p.rdLabel.meshFor(sel), at(lx, y))
			draw(val.meshFor(sel), at(lx+p.rdLabel.width+gap, y))
			if sel {
				drawArrow(lx, y)
			}
		}

		// Item 3: Apply
		{
			i := 3
			y := startY + float32(i)*lineH
			sel := p.selIndex == i
			x := (sw - p.apply.width) / 2
			if p.dirty {
				draw(p.apply.meshFor(sel), at(x, y))
			} else if p.apply.dim != nil {
				draw(p.apply.dim, at(x, y))
			} else {
				draw(p.apply.gray, at(x, y))
			}
			if sel {
				drawArrow(x, y)
			}
		}

		// Item 4: Back
		{
			i := 4
			y := startY + float32(i)*lineH
			sel := p.selIndex == i
			x := (sw - p.back.width) / 2
			draw(p.back.meshFor(sel), at(x, y))
			if sel {
				drawArrow(x, y)
			}
		}
	}

	r.EndUIPass(pass)
}

func (p *PauseMenu) Destroy(r *renderer.Renderer) {
	if p.overlay != nil {
		p.overlay.Destroy(r)
	}
	p.title.destroy(r)
	p.arrow.destroy(r)
	for i := range p.mainItems {
		p.mainItems[i].destroy(r)
	}
	p.settingsTitle.destroy(r)
	p.fsLabel.destroy(r)
	p.fsOn.destroy(r)
	p.fsOff.destroy(r)
	p.resLabel.destroy(r)
	p.rdLabel.destroy(r)
	p.apply.destroy(r)
	p.back.destroy(r)
	for i := range p.resOpts {
		p.resOpts[i].destroy(r)
	}
	for i := range p.rdOpts {
		p.rdOpts[i].destroy(r)
	}
}
