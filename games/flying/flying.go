package flying

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/toybox/games/calibration"
	"github.com/anthonyrego/toybox/pkg/control"
	"github.com/anthonyrego/toybox/pkg/engine"
	"github.com/anthonyrego/toybox/pkg/mesh"
	"github.com/anthonyrego/toybox/pkg/renderer"
	"github.com/anthonyrego/toybox/pkg/ui"
	"github.com/anthonyrego/toybox/wand"
)

const (
	flySpeed = float32(8.0) // forward units/s

	particleCount  = 3000
	spawnRadiusMin = float32(2.0)  // min distance from camera
	spawnRadiusMax = float32(60.0) // max distance from camera
	despawnRadius  = float32(65.0) // respawn when this far from camera
)

type flyParticle struct {
	X, Y, Z float32
	Size    float32
	R, G, B uint8
}

type Game struct {
	wand *wand.Listener
	time float32

	orientation mgl32.Quat
	position    mgl32.Vec3

	skyDome *mesh.Mesh

	particles []flyParticle
	partVB    *sdl.GPUBuffer
	partIB    *sdl.GPUBuffer

	// Calibration captures the neutral wand orientation on a web Calibrate
	// press, so the pose the wand is held in at calibration reads as "level".
	calib *calibration.Calibrator

	// Rings to fly through: a stream of hoops ahead of the player, each
	// rewarding a clean pass-through with a sparkle pop, a light pulse, and a
	// chime that climbs a scale with the combo streak.
	rings    []ring
	ringMesh []*mesh.Mesh // one prebuilt torus per palette color
	pops     []pop
	popVB    *sdl.GPUBuffer
	popIB    *sdl.GPUBuffer

	combo                  int
	lastHitTime            float32
	flash                  float32 // screen-flash intensity, decays to 0
	flashR, flashG, flashB uint8

	stream *sdl.AudioStream
	voices []chimeVoice
	mixBuf []float32

	debugMeshes [3]*mesh.Mesh
}

func New(w *wand.Listener) *Game {
	return &Game{wand: w, calib: calibration.New()}
}

func (g *Game) Init(e *engine.Engine) error {
	e.SetMouseMode(false)

	e.Cam.Position = mgl32.Vec3{0, 0, 0}
	e.Cam.Far = 100

	// Sky dome. Radius sits just inside the camera far plane (100) so the rings,
	// which are drawn first and write depth, always render in front of it.
	const (
		sphereRadius   = 95.0
		sphereRings    = 16
		sphereSegments = 24
	)

	type color3 struct{ r, g, b float64 }
	axisColors := [6]color3{
		{220, 130, 30}, // +X Orange
		{160, 50, 200}, // -X Purple
		{50, 100, 220}, // +Y Blue
		{220, 200, 50}, // -Y Yellow
		{220, 50, 50},  // +Z Red
		{50, 180, 50},  // -Z Green
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
	g.skyDome = &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(sphereIdxs))}

	// Lighting
	e.LightUniforms.AmbientColor = mgl32.Vec4{0.8, 0.8, 0.8, 1.0}
	e.LightUniforms.SunDirection = mgl32.Vec4{0, 0, -1, 0}
	e.LightUniforms.SunColor = mgl32.Vec4{1.0, 1.0, 1.0, 0.2}

	// Post-process
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0, 0, 0, 0},
		Tint:   mgl32.Vec4{1.0, 1.0, 1.0, 0},
	}

	// Orientation: identity quaternion, facing -Z (matches camera default)
	g.orientation = mgl32.QuatIdent()
	g.position = mgl32.Vec3{0, 0, 0}

	// Particles — distributed in a sphere around camera
	g.particles = make([]flyParticle, particleCount)
	for i := range g.particles {
		g.spawnParticle(&g.particles[i])
	}

	// Rings to fly through (built after orientation/position are set).
	if err := g.initRings(e); err != nil {
		return err
	}

	// Audio for the pass-through chimes (best-effort; game runs muted on failure).
	if e.Audio != nil {
		stream, err := e.Audio.NewStream()
		if err != nil {
			fmt.Println("Warning: flying audio:", err)
		} else {
			g.stream = stream
		}
	}

	return nil
}

func (g *Game) Update(e *engine.Engine, dt float32) bool {
	g.time += dt

	// Read wand state as a quaternion.
	s := g.wand.State()
	wq := mgl32.Quat{W: s.Q.W, V: mgl32.Vec3{s.Q.X, s.Q.Y, s.Q.Z}}

	// Direct tilt-to-fly: the wand's calibrated orientation (already in the
	// engine's camera-aligned frame from calib.Track) IS the plane's attitude,
	// so the same wand pose always yields the same heading — predictable, with no
	// integrated rate drift. Track returns identity until the player calibrates,
	// so the plane sits level until then.
	g.orientation = g.calib.Track(wq)

	// Move forward
	forward := g.orientation.Rotate(mgl32.Vec3{0, 0, -1})
	prevPos := g.position
	g.position = g.position.Add(forward.Mul(flySpeed * dt))

	// Sync camera position for engine light/fog uniforms
	e.Cam.Position = g.position

	// Update particles — respawn when too far from camera
	for i := range g.particles {
		p := &g.particles[i]
		dx := p.X - g.position[0]
		dy := p.Y - g.position[1]
		dz := p.Z - g.position[2]
		distSq := dx*dx + dy*dy + dz*dz
		if distSq > despawnRadius*despawnRadius {
			g.spawnParticle(p)
		}
	}

	// Rings: detect pass-throughs (using the pre-move position for the segment
	// test), advance pops, and drive the point-light pulses.
	g.updateRings(e, dt, prevPos)

	// Screen flash on a clear: briefly brighten in the ring's color, decaying
	// back to the neutral {1,1,1} tint set in Init.
	boost := g.flash * 0.6
	e.PostProcess.Tint = mgl32.Vec4{
		1 + boost*float32(g.flashR)/255,
		1 + boost*float32(g.flashG)/255,
		1 + boost*float32(g.flashB)/255,
		0,
	}

	g.generateAudio(dt)

	return true
}

func (g *Game) Render(e *engine.Engine, frame renderer.RenderFrame) {
	// Build custom ViewProj from quaternion orientation (supports roll)
	rotMatrix := g.orientation.Mat4()
	invRot := rotMatrix.Transpose()
	viewMatrix := invRot.Mul4(mgl32.Translate3D(
		-g.position[0], -g.position[1], -g.position[2],
	))
	proj := e.Cam.ProjectionMatrix()
	viewProj := proj.Mul4(viewMatrix)

	// Camera basis vectors for billboarding (from rotation matrix columns)
	camRight := mgl32.Vec3{rotMatrix[0], rotMatrix[1], rotMatrix[2]}
	camUp := mgl32.Vec3{rotMatrix[4], rotMatrix[5], rotMatrix[6]}

	// Solid rings first (lit pipeline). Drawing them before any swirl/fireball
	// draw keeps the lit pipeline, its samplers, and the engine's light uniforms
	// — all bound by BeginScenePass — valid, the same "lit draw first" order drum
	// circle relies on. They write depth, so the dome and stars occlude correctly.
	g.renderRings(e, frame, viewProj)

	// Sky dome (background). Its radius sits just inside the far plane so every
	// ring is nearer than it and stays visible in front; it writes no depth, so
	// it fills only the pixels the rings didn't.
	skyViewProj := proj.Mul4(invRot) // strip translation so dome is always centered
	skyModel := mgl32.Ident4()
	e.Rend.DrawSwirl(frame.CmdBuf, frame.ScenePass, renderer.SwirlDrawCall{
		VertexBuffer: g.skyDome.VertexBuffer,
		IndexBuffer:  g.skyDome.IndexBuffer,
		IndexCount:   g.skyDome.IndexCount,
		MVP:          skyViewProj.Mul4(skyModel),
		Model:        skyModel,
		Time:         g.time,
	})

	// Particles/stars (additive blend)
	g.renderParticles(e, frame, viewProj, camRight, camUp)

	// Pass-through celebrations on top (additive)
	g.renderPops(e, frame, viewProj, camRight, camUp)
}

func (g *Game) renderParticles(e *engine.Engine, frame renderer.RenderFrame, viewProj mgl32.Mat4, right, up mgl32.Vec3) {
	if g.partVB != nil {
		e.Rend.ReleaseBuffer(g.partVB)
		g.partVB = nil
	}
	if g.partIB != nil {
		e.Rend.ReleaseBuffer(g.partIB)
		g.partIB = nil
	}

	n := len(g.particles)
	vertices := make([]renderer.LitVertex, 0, n*4)
	indices := make([]uint16, 0, n*6)

	for i := range g.particles {
		p := &g.particles[i]

		rx := right[0] * p.Size
		ry := right[1] * p.Size
		rz := right[2] * p.Size
		ux := up[0] * p.Size
		uy := up[1] * p.Size
		uz := up[2] * p.Size

		base := uint16(len(vertices))

		vertices = append(vertices,
			renderer.LitVertex{
				X: p.X - rx - ux, Y: p.Y - ry - uy, Z: p.Z - rz - uz,
				R: p.R, G: p.G, B: p.B, A: 255,
				U: 0, V: 0,
			},
			renderer.LitVertex{
				X: p.X + rx - ux, Y: p.Y + ry - uy, Z: p.Z + rz - uz,
				R: p.R, G: p.G, B: p.B, A: 255,
				U: 1, V: 0,
			},
			renderer.LitVertex{
				X: p.X + rx + ux, Y: p.Y + ry + uy, Z: p.Z + rz + uz,
				R: p.R, G: p.G, B: p.B, A: 255,
				U: 1, V: 1,
			},
			renderer.LitVertex{
				X: p.X - rx + ux, Y: p.Y - ry + uy, Z: p.Z - rz + uz,
				R: p.R, G: p.G, B: p.B, A: 255,
				U: 0, V: 1,
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

	g.partVB = vb
	g.partIB = ib

	e.Rend.DrawFireball(frame.CmdBuf, frame.ScenePass, renderer.FireballDrawCall{
		VertexBuffer: g.partVB,
		IndexBuffer:  g.partIB,
		IndexCount:   uint32(len(indices)),
		MVP:          viewProj,
		Time:         g.time,
	})
}

func (g *Game) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
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
		y := float32(10) + float32(i)*ps*11
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
	g.skyDome.Destroy(e.Rend)
	if g.partVB != nil {
		e.Rend.ReleaseBuffer(g.partVB)
	}
	if g.partIB != nil {
		e.Rend.ReleaseBuffer(g.partIB)
	}
	for _, m := range g.ringMesh {
		if m != nil {
			m.Destroy(e.Rend)
		}
	}
	if g.popVB != nil {
		e.Rend.ReleaseBuffer(g.popVB)
	}
	if g.popIB != nil {
		e.Rend.ReleaseBuffer(g.popIB)
	}
	if g.stream != nil {
		g.stream.Destroy()
	}
	for i, m := range g.debugMeshes {
		if m != nil {
			m.Destroy(e.Rend)
			g.debugMeshes[i] = nil
		}
	}
}

// WebModule returns the game's control page: a single Calibrate button.
func (g *Game) WebModule() control.GameModule {
	return control.NewSettingsModule("FLYING", g.calib)
}

// NeedsCalibration reports whether the player has yet to calibrate the wand.
func (g *Game) NeedsCalibration() bool { return !g.calib.Calibrated() }

func (g *Game) spawnParticle(p *flyParticle) {
	// Random point on unit sphere (uniform distribution)
	for {
		x := rand.Float32()*2 - 1
		y := rand.Float32()*2 - 1
		z := rand.Float32()*2 - 1
		lenSq := x*x + y*y + z*z
		if lenSq < 0.01 || lenSq > 1.0 {
			continue
		}
		inv := float32(1.0 / math.Sqrt(float64(lenSq)))
		dist := spawnRadiusMin + rand.Float32()*(spawnRadiusMax-spawnRadiusMin)
		p.X = g.position[0] + x*inv*dist
		p.Y = g.position[1] + y*inv*dist
		p.Z = g.position[2] + z*inv*dist
		break
	}
	// Scale size with distance so far particles are still visible
	dx := p.X - g.position[0]
	dy := p.Y - g.position[1]
	dz := p.Z - g.position[2]
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy + dz*dz)))
	distT := (dist - spawnRadiusMin) / (spawnRadiusMax - spawnRadiusMin)
	if distT < 0 {
		distT = 0
	}
	if distT > 1 {
		distT = 1
	}
	p.Size = 0.05 + distT*0.1 + rand.Float32()*0.03

	// Random vibrant color
	palette := [][3]uint8{
		{255, 80, 20},   // orange
		{255, 40, 80},   // red-pink
		{200, 50, 255},  // purple
		{50, 120, 255},  // blue
		{40, 220, 200},  // cyan
		{255, 200, 30},  // gold
		{80, 255, 80},   // green
		{255, 120, 200}, // pink
	}
	c := palette[rand.Intn(len(palette))]
	p.R, p.G, p.B = c[0], c[1], c[2]
}
