package colorsphere

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/ui"
)

const numSamples = 300

const (
	particleCount = 300

	riverXMin     = float32(-3.0)
	riverXMax     = float32(3.0)
	riverYBase    = float32(0.0)
	riverYScatter = float32(0.3)
	riverZMin     = float32(-2.0)
	riverZMax     = float32(-5.0)

	flowSpeedMin = float32(0.3)
	flowSpeedMax = float32(0.8)

	sizeMin = float32(0.015)
	sizeMax = float32(0.045)

	waveAmplitude = float32(0.6)
	maxAccel      = float32(15.0) // linear accel magnitudes are smaller (gravity removed)
	lerpFactor    = float32(6.0)
)

type riverParticle struct {
	X, Y, Z    float32
	BaseY      float32
	VelX       float32
	Size       float32
	Brightness uint8
	displY     float32
}

type Game struct {
	wand     *wand.Listener
	pause    *ui.PauseMenu
	rotation mgl32.Mat4
	time     float32

	tunnel *mesh.Mesh

	particles   []riverParticle
	riverVB     *sdl.GPUBuffer
	riverIB     *sdl.GPUBuffer
	riverIdxCnt uint32

	samples  [numSamples]float32
	writePos int
	filled   bool

	// Neutral wand orientation (captured on first update). The sphere's
	// rotation is the body-frame delta from neutral — so whatever pose the
	// wand is in when the game starts becomes the sphere's "zero."
	neutralQ   mgl32.Quat
	calibrated bool

	wantsChange bool

	debugMeshes [3]*mesh.Mesh
}

func New(w *wand.Listener) *Game {
	return &Game{wand: w}
}

func (g *Game) WantsChangeGame() bool {
	return g.wantsChange
}

func (g *Game) Init(e *engine.Engine) error {
	g.wantsChange = false

	e.SetMouseMode(false)

	e.Cam.Position = mgl32.Vec3{0, 0, 0}
	e.Cam.Far = 100

	// Color sphere — 6 color regions mapped to axis directions
	const (
		sphereRadius   = 30.0
		sphereRings    = 16
		sphereSegments = 24
	)

	type color3 struct{ r, g, b float64 }
	axisColors := [6]color3{
		{220, 130, 30},  // +X Orange
		{160, 50, 200},  // -X Purple
		{50, 100, 220},  // +Y Blue
		{220, 200, 50},  // -Y Yellow
		{220, 50, 50},   // +Z Red
		{50, 180, 50},   // -Z Green
	}

	var sphereVerts []renderer.LitVertex
	var sphereIdxs []uint16

	for ring := 0; ring <= sphereRings; ring++ {
		phi := math.Pi * (1.0 - float64(ring)/float64(sphereRings))
		ny := math.Cos(phi)
		ringR := math.Sin(phi)

		for seg := 0; seg <= sphereSegments; seg++ {
			theta := 2 * math.Pi * float64(seg) / float64(sphereSegments)
			nx := ringR * math.Sin(theta)
			nz := ringR * math.Cos(theta)

			weights := [6]float64{
				math.Max(0, nx) * math.Max(0, nx),
				math.Max(0, -nx) * math.Max(0, -nx),
				math.Max(0, ny) * math.Max(0, ny),
				math.Max(0, -ny) * math.Max(0, -ny),
				math.Max(0, nz) * math.Max(0, nz),
				math.Max(0, -nz) * math.Max(0, -nz),
			}
			var totalW float64
			for _, w := range weights {
				totalW += w
			}

			var cr, cg, cb float64
			for i, w := range weights {
				f := w / totalW
				cr += axisColors[i].r * f
				cg += axisColors[i].g * f
				cb += axisColors[i].b * f
			}

			sphereVerts = append(sphereVerts, renderer.LitVertex{
				X:  float32(nx * sphereRadius),
				Y:  float32(ny * sphereRadius),
				Z:  float32(nz * sphereRadius),
				NX: float32(nx), NY: float32(ny), NZ: float32(nz),
				R: uint8(cr), G: uint8(cg), B: uint8(cb), A: 255,
			})
		}
	}

	for ring := 0; ring < sphereRings; ring++ {
		for seg := 0; seg < sphereSegments; seg++ {
			curr := uint16(ring*(sphereSegments+1) + seg)
			next := curr + uint16(sphereSegments+1)

			sphereIdxs = append(sphereIdxs, curr, next, curr+1)
			sphereIdxs = append(sphereIdxs, curr+1, next, next+1)
		}
	}

	vb, err := e.Rend.CreateLitVertexBuffer(sphereVerts)
	if err != nil {
		return fmt.Errorf("sphere vertex buffer: %w", err)
	}
	ib, err := e.Rend.CreateIndexBuffer(sphereIdxs)
	if err != nil {
		return fmt.Errorf("sphere index buffer: %w", err)
	}
	g.tunnel = &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(sphereIdxs))}

	// Pause menu
	resolutions := e.Win.DisplayModes()
	g.pause = ui.NewPauseMenu(e.Rend, resolutions, e.Win.SupportsHDR())
	startResIdx := 0
	for i, res := range resolutions {
		if res.W == e.Win.Width() && res.H == e.Win.Height() {
			startResIdx = i
			break
		}
	}
	startRDIdx := 0
	for i, v := range ui.RenderDistances {
		if float32(v) == e.Cam.Far {
			startRDIdx = i
			break
		}
	}
	g.pause.SetAppliedState(e.Win.IsFullscreen(), startResIdx, startRDIdx, e.Win.HDR())

	// Lighting
	if e.Win.HDR() {
		e.LightUniforms.AmbientColor = mgl32.Vec4{0.3, 0.3, 0.3, 1.0}
		e.LightUniforms.SunDirection = mgl32.Vec4{0, 0, -1, 0}
		e.LightUniforms.SunColor = mgl32.Vec4{1.0, 1.0, 1.0, 2.0}
	} else {
		e.LightUniforms.AmbientColor = mgl32.Vec4{0.8, 0.8, 0.8, 1.0}
		e.LightUniforms.SunDirection = mgl32.Vec4{0, 0, -1, 0}
		e.LightUniforms.SunColor = mgl32.Vec4{1.0, 1.0, 1.0, 0.2}
	}

	// Post-process
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0, 0, 0, 0},
		Tint:   mgl32.Vec4{1.0, 1.0, 1.0, 0},
	}

	g.rotation = mgl32.Ident4()
	g.calibrated = false

	// Particle river
	g.particles = make([]riverParticle, particleCount)
	for i := range g.particles {
		g.spawnParticle(&g.particles[i], true)
	}

	return nil
}

func (g *Game) Update(e *engine.Engine, dt float32) bool {
	action := g.pause.HandleInput(e.Input)
	switch action {
	case ui.ActionQuit:
		return false
	case ui.ActionApplySettings:
		fs := g.pause.PendingFullscreen()
		w, h := g.pause.PendingResolution()
		rd := g.pause.PendingRenderDistance()
		hdr := g.pause.PendingHDR()
		e.ApplyDisplaySettings(fs, w, h, rd, hdr)
		g.pause.ConfirmApply()
	case ui.ActionChangeGame:
		g.wantsChange = true
	}

	g.time += dt

	if !g.pause.IsActive() {
		s := g.wand.State()

		wq := mgl32.Quat{W: s.Q.W, V: mgl32.Vec3{s.Q.X, s.Q.Y, s.Q.Z}}
		if !g.calibrated {
			g.neutralQ = wq
			g.calibrated = true
		}
		qRel := g.neutralQ.Inverse().Mul(wq)
		g.rotation = qRel.Mat4()

		mag := float32(math.Sqrt(float64(s.LinAccelX*s.LinAccelX + s.LinAccelY*s.LinAccelY + s.LinAccelZ*s.LinAccelZ)))
		g.samples[g.writePos] = mag
		g.writePos++
		if g.writePos >= numSamples {
			g.writePos = 0
			g.filled = true
		}

		alpha := float32(1.0) - float32(math.Exp(float64(-lerpFactor*dt)))
		for i := range g.particles {
			p := &g.particles[i]
			p.X += p.VelX * dt
			if p.X > riverXMax+0.5 {
				g.spawnParticle(p, false)
			}
			targetY := g.sampleWaveAtX(p.X)
			p.displY += (targetY - p.displY) * alpha
			p.Y = p.BaseY + p.displY
		}
	}

	return true
}

func (g *Game) Render(e *engine.Engine, frame renderer.RenderFrame) {
	// Particle river first (uses lit pipeline which is already bound)
	g.renderParticleRiver(e, frame)

	// Swirl sphere last (switches to swirl pipeline)
	tunnelModel := g.rotation
	tunnelMVP := frame.ViewProj.Mul4(tunnelModel)
	e.Rend.DrawSwirl(frame.CmdBuf, frame.ScenePass, renderer.SwirlDrawCall{
		VertexBuffer: g.tunnel.VertexBuffer,
		IndexBuffer:  g.tunnel.IndexBuffer,
		IndexCount:   g.tunnel.IndexCount,
		MVP:          tunnelMVP,
		Model:        tunnelModel,
		Time:         g.time,
	})
}

func (g *Game) renderParticleRiver(e *engine.Engine, frame renderer.RenderFrame) {
	if g.riverVB != nil {
		e.Rend.ReleaseBuffer(g.riverVB)
		g.riverVB = nil
	}
	if g.riverIB != nil {
		e.Rend.ReleaseBuffer(g.riverIB)
		g.riverIB = nil
	}

	n := len(g.particles)
	vertices := make([]renderer.LitVertex, 0, n*4)
	indices := make([]uint16, 0, n*6)

	right := frame.CamRight
	up := frame.CamUp

	for i := range g.particles {
		p := &g.particles[i]

		rx := right[0] * p.Size
		ry := right[1] * p.Size
		rz := right[2] * p.Size
		ux := up[0] * p.Size
		uy := up[1] * p.Size
		uz := up[2] * p.Size

		b := p.Brightness
		base := uint16(len(vertices))

		vertices = append(vertices,
			renderer.LitVertex{
				X: p.X - rx - ux, Y: p.Y - ry - uy, Z: p.Z - rz - uz,
				NX: 0, NY: 0, NZ: 1, R: b, G: b, B: b, A: 255,
			},
			renderer.LitVertex{
				X: p.X + rx - ux, Y: p.Y + ry - uy, Z: p.Z + rz - uz,
				NX: 0, NY: 0, NZ: 1, R: b, G: b, B: b, A: 255,
			},
			renderer.LitVertex{
				X: p.X + rx + ux, Y: p.Y + ry + uy, Z: p.Z + rz + uz,
				NX: 0, NY: 0, NZ: 1, R: b, G: b, B: b, A: 255,
			},
			renderer.LitVertex{
				X: p.X - rx + ux, Y: p.Y - ry + uy, Z: p.Z - rz + uz,
				NX: 0, NY: 0, NZ: 1, R: b, G: b, B: b, A: 255,
			},
		)

		indices = append(indices,
			base, base+1, base+2,
			base, base+2, base+3,
		)
	}

	vb, err := e.Rend.CreateLitVertexBuffer(vertices)
	if err != nil {
		return
	}
	ib, err := e.Rend.CreateIndexBuffer(indices)
	if err != nil {
		e.Rend.ReleaseBuffer(vb)
		return
	}

	g.riverVB = vb
	g.riverIB = ib
	g.riverIdxCnt = uint32(len(indices))

	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: g.riverVB,
		IndexBuffer:  g.riverIB,
		IndexCount:   g.riverIdxCnt,
		MVP:          frame.ViewProj,
		NoFog:        true,
	})
}

func (g *Game) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	if g.pause.IsActive() {
		g.pause.Render(e.Rend, cmdBuf, target, e.Win.Width(), e.Win.Height())
		return
	}

	// Release previous frame's debug meshes.
	for i, m := range g.debugMeshes {
		if m != nil {
			m.Destroy(e.Rend)
			g.debugMeshes[i] = nil
		}
	}

	s := g.wand.State()
	roll, pitch, yaw := s.Euler()
	lines := [3]string{
		fmt.Sprintf("ROLL  %7.1f", roll),
		fmt.Sprintf("PITCH %7.1f", pitch),
		fmt.Sprintf("YAW   %7.1f", yaw),
	}

	const ps = float32(3)
	ortho := mgl32.Ortho2D(0, float32(e.Win.Width()), float32(e.Win.Height()), 0)
	pass := e.Rend.BeginUIPass(cmdBuf, target)

	for i, text := range lines {
		m, _, err := ui.NewTextMesh(e.Rend, text, ps, 255, 255, 255, 200)
		if err != nil {
			continue
		}
		g.debugMeshes[i] = m
		y := float32(10) + float32(i)*ps*8
		e.Rend.DrawUI(cmdBuf, pass, renderer.DrawCall{
			VertexBuffer: m.VertexBuffer,
			IndexBuffer:  m.IndexBuffer,
			IndexCount:   m.IndexCount,
			Transform:    ortho.Mul4(mgl32.Translate3D(10, y, 0)),
		})
	}

	e.Rend.EndUIPass(pass)
}

func (g *Game) Destroy(e *engine.Engine) {
	g.pause.Destroy(e.Rend)
	g.tunnel.Destroy(e.Rend)
	if g.riverVB != nil {
		e.Rend.ReleaseBuffer(g.riverVB)
	}
	if g.riverIB != nil {
		e.Rend.ReleaseBuffer(g.riverIB)
	}
	for i, m := range g.debugMeshes {
		if m != nil {
			m.Destroy(e.Rend)
			g.debugMeshes[i] = nil
		}
	}
}

func (g *Game) spawnParticle(p *riverParticle, randomX bool) {
	if randomX {
		p.X = riverXMin + rand.Float32()*(riverXMax-riverXMin)
	} else {
		p.X = riverXMin - rand.Float32()*0.5
	}
	p.Z = riverZMin + rand.Float32()*(riverZMax-riverZMin)
	p.BaseY = riverYBase + (rand.Float32()*2-1)*riverYScatter
	p.Y = p.BaseY
	p.VelX = flowSpeedMin + rand.Float32()*(flowSpeedMax-flowSpeedMin)

	depthT := (p.Z - riverZMax) / (riverZMin - riverZMax)
	p.Size = sizeMin + depthT*(sizeMax-sizeMin) + rand.Float32()*0.005
	p.Brightness = uint8(100 + depthT*155)
	p.displY = 0
}

func (g *Game) sampleWaveAtX(x float32) float32 {
	sampleCount := g.writePos
	if g.filled {
		sampleCount = numSamples
	}
	if sampleCount < 2 {
		return 0
	}

	t := (x - riverXMin) / (riverXMax - riverXMin)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	age := t * float32(sampleCount-1)
	idx0 := int(age)
	idx1 := idx0 + 1
	if idx1 >= sampleCount {
		idx1 = sampleCount - 1
	}
	frac := age - float32(idx0)

	ri0 := (g.writePos - 1 - idx0 + numSamples) % numSamples
	ri1 := (g.writePos - 1 - idx1 + numSamples) % numSamples

	raw := g.samples[ri0] + (g.samples[ri1]-g.samples[ri0])*frac

	normalized := raw / maxAccel
	if normalized > 1 {
		normalized = 1
	}
	return (normalized - 0.5) * 2.0 * waveAmplitude
}
