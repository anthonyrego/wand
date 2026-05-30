// Package splash is the wand's boot/idle screen. It is shown at launch and
// whenever no game is running: the screen becomes a pure "connect to me" prompt
// — the personalized title, a QR code of the control URL, and the URL itself —
// so a parent opens the web UI on a phone and drives everything from there.
//
// There is no keyboard interaction here (Esc-to-quit is handled app-wide by the
// App). The QR is drawn as a flat quad through the lit pipeline's UV texture
// path, exactly like the flashcard images; the text is drawn in the 2D overlay.
package splash

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand/pkg/control"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/ui"
)

// uvInset keeps the top-left UV corner above the lit shader's texCoord > 0.001
// threshold so every fragment routes through the texture path.
const uvInset = 0.0015

// TitleProvider supplies the personalized screen title (e.g. "EMMA'S WAND") and
// a revision that changes whenever the title does, so the splash can rebuild its
// title text live when the name is edited from the web.
type TitleProvider interface {
	Title() string
	Revision() uint64
}

type Splash struct {
	titleProvider TitleProvider
	url           string

	ps       float32
	titleRev uint64
	title    *mesh.Mesh
	titleW   float32

	qrTex  *sdl.GPUTexture
	qrW    uint32
	qrH    uint32
	quad   *mesh.Mesh
	meshes []*mesh.Mesh // overlay text meshes to free next frame
}

func New(titleProvider TitleProvider, url string) *Splash {
	return &Splash{titleProvider: titleProvider, url: url}
}

func (s *Splash) titleText() string {
	if s.titleProvider != nil {
		return s.titleProvider.Title()
	}
	return "WAND"
}

func (s *Splash) Init(e *engine.Engine) error {
	e.SetMouseMode(false)
	s.ps = float32(ui.FontScale)

	// Camera: fixed, looking straight down -Z at the QR plane (z=0), matching
	// the flashcard setup so the textured quad reproduces exact colors.
	e.Cam.Position = mgl32.Vec3{0, 0, 5}
	e.Cam.Yaw = -90
	e.Cam.Pitch = 0
	e.Cam.FOV = 45
	e.Cam.Far = 100
	e.LightUniforms = renderer.LightUniforms{AmbientColor: mgl32.Vec4{1, 1, 1, 1}}
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0, 0, 0, 0},
		Tint:   mgl32.Vec4{1, 1, 1, 0},
	}

	if s.titleProvider != nil {
		s.titleRev = s.titleProvider.Revision()
	}
	m, w, err := ui.NewTextMesh(e.Rend, s.titleText(), s.ps, 255, 255, 255, 255)
	if err != nil {
		return err
	}
	s.title, s.titleW = m, w

	// Encode the connect URL as a QR and upload it as a texture (CPU encode in
	// pkg/control, GPU upload here on the loop thread).
	if qw, qh, pix, err := control.QRCodeRGBA(s.url, 512); err == nil {
		if tex, err := e.Rend.CreateTextureFromRGBA(qw, qh, pix); err == nil {
			s.qrTex, s.qrW, s.qrH = tex, qw, qh
		}
	}

	return nil
}

// refreshTitle rebuilds the title mesh when the owner name changes (a parent
// renamed the wand from the web while the splash is up).
func (s *Splash) refreshTitle(e *engine.Engine) {
	if s.titleProvider == nil {
		return
	}
	rev := s.titleProvider.Revision()
	if rev == s.titleRev {
		return
	}
	m, w, err := ui.NewTextMesh(e.Rend, s.titleText(), s.ps, 255, 255, 255, 255)
	if err != nil {
		return
	}
	if s.title != nil {
		s.title.Destroy(e.Rend)
	}
	s.title, s.titleW = m, w
	s.titleRev = rev
}

func (s *Splash) Update(e *engine.Engine, dt float32) bool {
	s.refreshTitle(e)
	return true
}

func (s *Splash) Render(e *engine.Engine, frame renderer.RenderFrame) {
	if s.quad != nil {
		s.quad.Destroy(e.Rend)
		s.quad = nil
	}
	if s.qrTex == nil {
		return
	}
	q := s.buildQuad(e)
	if q == nil {
		return
	}
	s.quad = q
	e.Rend.BindLitTexture(frame.ScenePass, s.qrTex)
	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: q.VertexBuffer,
		IndexBuffer:  q.IndexBuffer,
		IndexCount:   q.IndexCount,
		MVP:          frame.ViewProj,
		Model:        mgl32.Ident4(),
		NoFog:        true,
	})
}

// buildQuad makes a centered square quad at z=0, sized to a comfortable fraction
// of the visible height with room above for the title and below for the URL.
func (s *Splash) buildQuad(e *engine.Engine) *mesh.Mesh {
	dist := float64(e.Cam.Position.Z())
	halfH := dist * math.Tan(float64(mgl32.DegToRad(e.Cam.FOV))/2)

	side := float32(halfH * 0.95) // square side in world units
	half := side / 2
	cy := float32(halfH * 0.10) // nudge up slightly to balance the URL line

	verts := []renderer.LitVertex{
		{X: -half, Y: cy - half, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: uvInset, V: 1},
		{X: half, Y: cy - half, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: 1, V: 1},
		{X: half, Y: cy + half, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: 1, V: uvInset},
		{X: -half, Y: cy + half, Z: 0, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255, U: uvInset, V: uvInset},
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

func (s *Splash) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	for _, m := range s.meshes {
		m.Destroy(e.Rend)
	}
	s.meshes = s.meshes[:0]

	w := float32(e.Win.Width())
	h := float32(e.Win.Height())
	ortho := mgl32.Ortho2D(0, w, h, 0)
	pass := e.Rend.BeginUIPass(cmdBuf, target)

	// Title near the top.
	if s.title != nil {
		x := (w - s.titleW) / 2
		e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
			VertexBuffer: s.title.VertexBuffer,
			IndexBuffer:  s.title.IndexBuffer,
			IndexCount:   s.title.IndexCount,
			Transform:    ortho.Mul4(mgl32.Translate3D(x, h*0.07, 0)),
		})
	}

	// The connect URL below the QR (shrinks to fit on longer addresses).
	big := fitPixelSize(s.url, h/120, w*0.92)
	s.drawCenteredText(e, cmdBuf, pass, ortho, s.url, w, h*0.84, big, 240, 175, 95)

	e.Rend.EndUIPass(pass)
}

func (s *Splash) drawCenteredText(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, pass *sdl.GPURenderPass, ortho mgl32.Mat4, text string, screenW, y, ps float32, r, g, b uint8) {
	m, width, err := ui.NewTextMesh(e.Rend, text, ps, r, g, b, 255)
	if err != nil {
		return
	}
	s.meshes = append(s.meshes, m)
	x := (screenW - width) / 2
	e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
		VertexBuffer: m.VertexBuffer,
		IndexBuffer:  m.IndexBuffer,
		IndexCount:   m.IndexCount,
		Transform:    ortho.Mul4(mgl32.Translate3D(x, y, 0)),
	})
}

// fitPixelSize shrinks the desired pixel size so the text fits within maxWidth.
func fitPixelSize(text string, desired, maxWidth float32) float32 {
	if wpx := ui.TextWidth(text, desired); wpx > maxWidth && wpx > 0 {
		return desired * maxWidth / wpx
	}
	return desired
}

func (s *Splash) Destroy(e *engine.Engine) {
	if s.title != nil {
		s.title.Destroy(e.Rend)
		s.title = nil
	}
	if s.quad != nil {
		s.quad.Destroy(e.Rend)
		s.quad = nil
	}
	for _, m := range s.meshes {
		m.Destroy(e.Rend)
	}
	s.meshes = nil
	if s.qrTex != nil {
		e.Rend.ReleaseTexture(s.qrTex)
		s.qrTex = nil
	}
}
