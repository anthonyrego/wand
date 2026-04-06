package selector

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/sign"
	"github.com/anthonyrego/wand/pkg/ui"
)

type textEntry struct {
	mesh  *mesh.Mesh
	gray  *mesh.Mesh
	width float32
}

type Selector struct {
	names    []string
	selected int
	chosen   int // -1 until user presses Enter

	ps      float32
	title   textEntry
	arrow   textEntry
	items   []textEntry
	overlay *mesh.Mesh
}

func New(names []string) *Selector {
	return &Selector{names: names, chosen: -1}
}

func (s *Selector) Chosen() int  { return s.chosen }
func (s *Selector) Reset()       { s.chosen = -1; s.selected = 0 }

func (s *Selector) Init(e *engine.Engine) error {
	e.SetMouseMode(false)

	s.chosen = -1
	s.selected = 0
	s.ps = float32(e.PixelScale)

	r := e.Rend

	// Title
	m, w, err := ui.NewTextMesh(r, "WAND", s.ps, 255, 255, 255, 255)
	if err != nil {
		return err
	}
	s.title = textEntry{mesh: m, width: w}

	// Arrow
	m, w, err = ui.NewTextMesh(r, ">", s.ps, 255, 255, 255, 255)
	if err != nil {
		return err
	}
	s.arrow = textEntry{mesh: m, width: w}

	// Game items
	s.items = make([]textEntry, len(s.names))
	for i, name := range s.names {
		white, ww, err := ui.NewTextMesh(r, name, s.ps, 255, 255, 255, 255)
		if err != nil {
			return err
		}
		gray, _, err := ui.NewTextMesh(r, name, s.ps, 120, 120, 120, 255)
		if err != nil {
			white.Destroy(r)
			return err
		}
		s.items[i] = textEntry{mesh: white, gray: gray, width: ww}
	}

	// Dark overlay quad
	overlayVerts := []renderer.Vertex{
		{X: 0, Y: 0, Z: 0, R: 0, G: 0, B: 0, A: 220},
		{X: 1, Y: 0, Z: 0, R: 0, G: 0, B: 0, A: 220},
		{X: 1, Y: 1, Z: 0, R: 0, G: 0, B: 0, A: 220},
		{X: 0, Y: 1, Z: 0, R: 0, G: 0, B: 0, A: 220},
	}
	overlayIdx := []uint16{0, 1, 2, 0, 2, 3}
	vb, err := r.CreateVertexBuffer(overlayVerts)
	if err != nil {
		return err
	}
	ib, err := r.CreateIndexBuffer(overlayIdx)
	if err != nil {
		r.ReleaseBuffer(vb)
		return err
	}
	s.overlay = &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: 6}

	// Minimal scene setup
	e.Cam.Position = mgl32.Vec3{0, 0, 0}
	e.Cam.Far = 10
	e.LightUniforms.AmbientColor = mgl32.Vec4{0, 0, 0, 1}
	e.PostProcess = renderer.PostProcessUniforms{
		Tint: mgl32.Vec4{0, 0, 0, 0},
	}

	return nil
}

func (s *Selector) Update(e *engine.Engine, dt float32) bool {
	if len(s.items) == 0 {
		return true
	}

	inp := e.Input
	if inp.IsKeyPressed(sdl.K_UP) {
		s.selected--
		if s.selected < 0 {
			s.selected = len(s.items) - 1
		}
	}
	if inp.IsKeyPressed(sdl.K_DOWN) {
		s.selected++
		if s.selected >= len(s.items) {
			s.selected = 0
		}
	}
	if inp.IsKeyPressed(sdl.K_RETURN) {
		s.chosen = s.selected
	}

	return true
}

func (s *Selector) Render(e *engine.Engine, frame renderer.RenderFrame) {
	// No 3D scene
}

func (s *Selector) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	r := e.Rend
	sw := float32(e.Win.Width())
	sh := float32(e.Win.Height())

	ortho := mgl32.Ortho2D(0, sw, sh, 0)
	pass := r.BeginUIPass(cmdBuf, target)

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

	at := func(x, y float32) mgl32.Mat4 {
		return ortho.Mul4(mgl32.Translate3D(x, y, 0))
	}

	// Dark background
	draw(s.overlay, ortho.Mul4(mgl32.Scale3D(sw, sh, 1)))

	charH := s.ps * float32(sign.CharHeight)
	lineH := charH + s.ps*3

	// Title
	titleY := sh*0.30 - charH/2
	draw(s.title.mesh, at((sw-s.title.width)/2, titleY))

	// Game list
	startY := sh * 0.45
	for i, item := range s.items {
		y := startY + float32(i)*lineH
		x := (sw - item.width) / 2
		if i == s.selected {
			draw(item.mesh, at(x, y))
			draw(s.arrow.mesh, at(x-s.arrow.width-s.ps*2, y))
		} else {
			draw(item.gray, at(x, y))
		}
	}

	r.EndUIPass(pass)
}

func (s *Selector) Destroy(e *engine.Engine) {
	r := e.Rend
	if s.title.mesh != nil {
		s.title.mesh.Destroy(r)
	}
	if s.arrow.mesh != nil {
		s.arrow.mesh.Destroy(r)
	}
	for _, item := range s.items {
		if item.mesh != nil {
			item.mesh.Destroy(r)
		}
		if item.gray != nil {
			item.gray.Destroy(r)
		}
	}
	if s.overlay != nil {
		s.overlay.Destroy(r)
	}
}
