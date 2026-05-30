// Package flashcard is a wand game that shows a single large image with its
// name, managed live from any device on the local network via a built-in web
// server. A parent uploads cards (image + name) and chooses which one is on
// screen; the deck persists to disk between runs.
//
// The wand is unused here — this game is a screen plus a web-managed library.
//
// Rendering: each card is a flat quad drawn through the lit pipeline's UV
// texture path (see Renderer.BindLitTexture and Lit.frag). Lighting is set to
// pure white ambient with no sun or point lights, so the quad reproduces the
// image's colors exactly. The card name is drawn as crisp full-resolution text
// in the 2D overlay pass.
package flashcard

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/pkg/control"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/ui"
)

// uvInset keeps the top-left UV corner just above the lit shader's
// texCoord > 0.001 threshold so every fragment routes through the texture path
// (the (0,0) corner would otherwise fall back to the vertex-color path).
const uvInset = 0.0015

type cardTex struct {
	tex  *sdl.GPUTexture
	w, h uint32
}

type Game struct {
	wand *wand.Listener // unused; kept for constructor compatibility

	store  *store
	module *module

	textures     map[string]*cardTex // photo id -> uploaded GPU texture
	order        []Card              // cached deck order (set during reconcile)
	currentID    string
	currentPhoto int
	seenRev      uint64

	quad          *mesh.Mesh   // current card's quad, rebuilt each frame
	overlayMeshes []*mesh.Mesh // text meshes to free next overlay
}

func New(w *wand.Listener) *Game {
	return &Game{wand: w, textures: map[string]*cardTex{}}
}

// WebModule exposes the deck-management SPA + REST on the control server while
// the game is active.
func (g *Game) WebModule() control.GameModule {
	return g.module
}

func (g *Game) Init(e *engine.Engine) error {
	e.SetMouseMode(false)

	// Camera: fixed, looking straight down -Z at the card plane (z=0).
	e.Cam.Position = mgl32.Vec3{0, 0, 5}
	e.Cam.Yaw = -90
	e.Cam.Pitch = 0
	e.Cam.FOV = 45
	e.Cam.Far = 100

	// Flat, unlit look: white ambient, no sun, no point lights. The lit
	// shader's UV path then yields exactly the texture's colors.
	e.LightUniforms = renderer.LightUniforms{
		AmbientColor: mgl32.Vec4{1, 1, 1, 1},
	}

	// No dithering — show the image cleanly.
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0, 0, 0, 0},
		Tint:   mgl32.Vec4{1, 1, 1, 0},
	}

	// Deck store + web module (the control server serves it; no listener here).
	st, err := newStore(deckDir())
	if err != nil {
		return fmt.Errorf("flashcard store: %w", err)
	}
	g.store = st
	g.module = newModule(st)

	return nil
}

func (g *Game) Update(e *engine.Engine, dt float32) bool {
	g.reconcile(e)
	return true
}

// reconcile syncs GPU state to the deck whenever it changes: it uploads textures
// for new cards, releases textures for removed cards, and caches the order and
// current selection for rendering. This is the only place GPU resources are
// created or destroyed, keeping all SDL calls on the game-loop thread.
func (g *Game) reconcile(e *engine.Engine) {
	snap, changed := g.store.snapshotIfChanged(g.seenRev)
	if !changed {
		return
	}
	g.seenRev = snap.revision
	g.order = snap.cards
	g.currentID = snap.currentID
	g.currentPhoto = snap.currentPhoto

	for id, d := range snap.uploads {
		tex, err := e.Rend.CreateTextureFromRGBA(d.W, d.H, d.Pix)
		if err != nil {
			fmt.Println("flashcard: upload texture:", err)
			continue
		}
		if old := g.textures[id]; old != nil {
			e.Rend.ReleaseTexture(old.tex)
		}
		g.textures[id] = &cardTex{tex: tex, w: d.W, h: d.H}
	}

	valid := make(map[string]bool, len(snap.cards))
	for _, c := range snap.cards {
		for _, p := range c.Photos {
			valid[p.ID] = true
		}
	}
	for id, ct := range g.textures {
		if !valid[id] {
			e.Rend.ReleaseTexture(ct.tex)
			delete(g.textures, id)
		}
	}
}

// currentTex resolves the GPU texture for the on-screen card's selected photo,
// or nil when there is nothing (or nothing uploaded yet) to show.
func (g *Game) currentTex() *cardTex {
	for _, c := range g.order {
		if c.ID != g.currentID {
			continue
		}
		if g.currentPhoto < 0 || g.currentPhoto >= len(c.Photos) {
			return nil
		}
		return g.textures[c.Photos[g.currentPhoto].ID]
	}
	return nil
}

func (g *Game) Render(e *engine.Engine, frame renderer.RenderFrame) {
	if g.quad != nil {
		g.quad.Destroy(e.Rend)
		g.quad = nil
	}

	ct := g.currentTex()
	if ct == nil {
		return
	}

	q := g.buildCardQuad(e, ct)
	if q == nil {
		return
	}
	g.quad = q

	e.Rend.BindLitTexture(frame.ScenePass, ct.tex)
	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: q.VertexBuffer,
		IndexBuffer:  q.IndexBuffer,
		IndexCount:   q.IndexCount,
		MVP:          frame.ViewProj,
		Model:        mgl32.Ident4(),
		NoFog:        true,
	})
}

// buildCardQuad makes a screen-facing quad at z=0 sized to fit the visible
// frustum while preserving the card image's aspect ratio.
func (g *Game) buildCardQuad(e *engine.Engine, ct *cardTex) *mesh.Mesh {
	dist := math.Abs(float64(e.Cam.Position.Z()))
	halfH := dist * math.Tan(float64(mgl32.DegToRad(e.Cam.FOV))/2)
	halfW := halfH * float64(e.Cam.AspectRatio)

	maxH := halfH * 1.5 // ~75% of visible height, leaving room for the caption
	maxW := halfW * 1.8 // ~90% of visible width
	cy := float32(halfH * 0.17)

	aspect := float64(ct.w) / float64(ct.h)
	qw, qh := maxW, maxW/aspect
	if qh > maxH {
		qh, qw = maxH, maxH*aspect
	}

	hw := float32(qw / 2)
	hh := float32(qh / 2)

	verts := []renderer.LitVertex{
		{X: -hw, Y: cy - hh, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: uvInset, V: 1},       // bottom-left
		{X: hw, Y: cy - hh, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: 1, V: 1},              // bottom-right
		{X: hw, Y: cy + hh, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: 1, V: uvInset},        // top-right
		{X: -hw, Y: cy + hh, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: uvInset, V: uvInset}, // top-left
	}
	idxs := []uint16{0, 1, 2, 0, 2, 3}

	vb, err := e.Rend.CreateLitVertexBuffer(verts)
	if err != nil {
		return nil
	}
	ib, err := e.Rend.CreateIndexBuffer(idxs)
	if err != nil {
		e.Rend.ReleaseBuffer(vb)
		return nil
	}
	return &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(idxs))}
}

func (g *Game) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	for _, m := range g.overlayMeshes {
		m.Destroy(e.Rend)
	}
	g.overlayMeshes = g.overlayMeshes[:0]

	w := float32(e.Win.Width())
	h := float32(e.Win.Height())
	ortho := mgl32.Ortho2D(0, w, h, 0)
	pass := e.Rend.BeginUIPass(cmdBuf, target)

	if ct := g.currentTex(); ct != nil {
		// A card is on screen: caption it near the bottom, centered.
		name := titleFirst(g.nameByID(g.currentID))
		ps := fitPixelSize(name, h/130, w*0.92)
		y := h - ui.FontRows*ps - h*0.04
		g.drawCenteredText(e, cmdBuf, pass, ortho, name, w, y, ps, 255, 255, 255)

		// When the card has multiple photos, show a small "2 / 5" indicator
		// above the caption so a parent can see where they are in the set.
		if n := g.photoCount(g.currentID); n > 1 {
			counter := fmt.Sprintf("%d / %d", g.currentPhoto+1, n)
			cps := fitPixelSize(counter, h/220, w*0.5)
			g.drawCenteredText(e, cmdBuf, pass, ortho, counter, w, y-ui.FontRows*cps-h*0.015, cps, 150, 150, 165)
		}
	} else {
		// No card yet: the parent is already on the web UI (they switched to this
		// game there), so just prompt them to add cards from the flashcards page.
		const line1 = "ADD CARDS FROM YOUR PHONE"
		const line2 = "ON THE FLASHCARDS PAGE"
		s1 := fitPixelSize(line1, h/150, w*0.92)
		s2 := fitPixelSize(line2, h/200, w*0.92)
		y0 := h * 0.44
		g.drawCenteredText(e, cmdBuf, pass, ortho, line1, w, y0, s1, 220, 200, 210)
		g.drawCenteredText(e, cmdBuf, pass, ortho, line2, w, y0+ui.FontRows*s1+h*0.025, s2, 150, 150, 165)
	}

	e.Rend.EndUIPass(pass)
}

// fitPixelSize returns the desired pixel size, shrunk if necessary so the text
// fits within maxWidth (keeps long names and URLs on screen).
func fitPixelSize(text string, desired, maxWidth float32) float32 {
	if wpx := ui.TextWidth(text, desired); wpx > maxWidth && wpx > 0 {
		return desired * maxWidth / wpx
	}
	return desired
}

// drawCenteredText renders one line of 8x8-font text horizontally centered at
// the given top y, in screen-space pixels.
func (g *Game) drawCenteredText(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, pass *sdl.GPURenderPass, ortho mgl32.Mat4, text string, screenW, y, ps float32, r, gg, b uint8) {
	m, width, err := ui.NewTextMesh(e.Rend, text, ps, r, gg, b, 255)
	if err != nil {
		return
	}
	g.overlayMeshes = append(g.overlayMeshes, m)
	x := (screenW - width) / 2
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: m.VertexBuffer,
		IndexBuffer:  m.IndexBuffer,
		IndexCount:   m.IndexCount,
		Transform:    ortho.Mul4(mgl32.Translate3D(x, y, 0)),
	})
}

func (g *Game) nameByID(id string) string {
	for _, c := range g.order {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func (g *Game) photoCount(id string) int {
	for _, c := range g.order {
		if c.ID == id {
			return len(c.Photos)
		}
	}
	return 0
}

// titleFirst capitalizes the first letter and leaves the rest as typed
// (e.g. "apple" -> "Apple", "APPLE" -> "APPLE").
func titleFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func (g *Game) Destroy(e *engine.Engine) {
	if g.quad != nil {
		g.quad.Destroy(e.Rend)
		g.quad = nil
	}
	for _, m := range g.overlayMeshes {
		m.Destroy(e.Rend)
	}
	g.overlayMeshes = nil
	for id, ct := range g.textures {
		e.Rend.ReleaseTexture(ct.tex)
		delete(g.textures, id)
	}
}

func deckDir() string {
	if d := os.Getenv("WAND_FLASHCARD_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "flashcards"
	}
	return filepath.Join(home, ".wand", "flashcards")
}
