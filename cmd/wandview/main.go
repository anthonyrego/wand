package main

import (
	"fmt"
	"math"
	"os"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/construct/pkg/engine"
	"github.com/anthonyrego/construct/pkg/mesh"
	"github.com/anthonyrego/construct/pkg/renderer"
	"github.com/anthonyrego/construct/pkg/settings"
	"github.com/anthonyrego/construct/pkg/ui"

	"github.com/anthonyrego/wand"
)

const numSamples = 300

type WandViewGame struct {
	wand     *wand.Listener
	pause    *ui.PauseMenu
	rotation mgl32.Mat4

	tunnel *mesh.Mesh

	waveVB     *sdl.GPUBuffer
	waveIB     *sdl.GPUBuffer
	waveIdxCnt uint32

	samples  [numSamples]float32
	writePos int
	filled   bool
}

func main() {
	err := sdl.LoadLibrary(sdl.Path())
	if err != nil {
		fmt.Println("Loading embedded SDL3 library...")
		defer binsdl.Load().Unload()
	}

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		panic("failed to initialize SDL: " + err.Error())
	}
	defer sdl.Quit()

	ds := settings.Default()

	e, err := engine.New("Wand View", ds)
	if err != nil {
		panic(err)
	}
	defer e.Destroy()

	if err := e.Run(&WandViewGame{}); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func (g *WandViewGame) Init(e *engine.Engine) error {
	e.SetMouseMode(false)

	e.Cam.Position = mgl32.Vec3{0, 0, 0}
	e.Cam.Far = 100

	// Color sphere — 6 color regions mapped to axis directions
	const (
		sphereRadius   = 30.0
		sphereRings    = 16
		sphereSegments = 24
	)

	// Colors for each axis direction: +X, -X, +Y, -Y, +Z, -Z
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

			// Blend colors by dot product with each axis direction (squared for sharper regions)
			weights := [6]float64{
				math.Max(0, nx) * math.Max(0, nx),   // +X
				math.Max(0, -nx) * math.Max(0, -nx), // -X
				math.Max(0, ny) * math.Max(0, ny),   // +Y
				math.Max(0, -ny) * math.Max(0, -ny), // -Y
				math.Max(0, nz) * math.Max(0, nz),   // +Z
				math.Max(0, -nz) * math.Max(0, -nz), // -Z
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
				NX: 0, NY: -1, NZ: 0,
				R: uint8(cr), G: uint8(cg), B: uint8(cb), A: 255,
			})
		}
	}

	for ring := 0; ring < sphereRings; ring++ {
		for seg := 0; seg < sphereSegments; seg++ {
			curr := uint16(ring*(sphereSegments+1) + seg)
			next := curr + uint16(sphereSegments+1)

			// Inward-facing winding (same as sky dome)
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

	// Wand listener
	g.wand = wand.New(9999)
	if err := g.wand.Start(); err != nil {
		return fmt.Errorf("wand: %w", err)
	}

	// Pause menu
	resolutions := e.Win.DisplayModes()
	g.pause = ui.NewPauseMenu(e.Rend, e.PixelScale, resolutions)
	startResIdx := 0
	for i, res := range resolutions {
		if res.W == e.Win.Width() && res.H == e.Win.Height() {
			startResIdx = i
			break
		}
	}
	startPSIdx := 0
	for i, v := range ui.PixelScales {
		if v == e.PixelScale {
			startPSIdx = i
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
	g.pause.SetAppliedState(e.Win.IsFullscreen(), startResIdx, startPSIdx, startRDIdx)

	// Lighting — high ambient so all tunnel faces are vivid
	e.LightUniforms.AmbientColor = mgl32.Vec4{0.8, 0.8, 0.8, 1.0}
	e.LightUniforms.SunDirection = mgl32.Vec4{0, 0, -1, 0}
	e.LightUniforms.SunColor = mgl32.Vec4{1.0, 1.0, 1.0, 0.2}

	// Post-process
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0, 0, 0, 0},
		Tint:   mgl32.Vec4{1.0, 1.0, 1.0, 0},
	}

	g.rotation = mgl32.Ident4()

	return nil
}

func (g *WandViewGame) Update(e *engine.Engine, dt float32) bool {
	action := g.pause.HandleInput(e.Input)
	switch action {
	case ui.ActionQuit:
		return false
	case ui.ActionApplySettings:
		fs := g.pause.PendingFullscreen()
		w, h := g.pause.PendingResolution()
		ps := g.pause.PendingPixelScale()
		rd := g.pause.PendingRenderDistance()
		e.ApplyDisplaySettings(fs, w, h, ps, rd)
		g.pause.ConfirmApply()
	}

	if !g.pause.IsActive() {
		s := g.wand.State()

		roll := mgl32.DegToRad(s.Roll)
		pitch := mgl32.DegToRad(s.Pitch)
		yaw := mgl32.DegToRad(s.Yaw)
		g.rotation = mgl32.HomogRotate3DY(yaw).Mul4(
			mgl32.HomogRotate3DX(pitch)).Mul4(
			mgl32.HomogRotate3DZ(roll))

		// Sample acceleration magnitude into ring buffer
		mag := float32(math.Sqrt(float64(s.AccelX*s.AccelX + s.AccelY*s.AccelY + s.AccelZ*s.AccelZ)))
		g.samples[g.writePos] = mag
		g.writePos++
		if g.writePos >= numSamples {
			g.writePos = 0
			g.filled = true
		}
	}

	return true
}

func (g *WandViewGame) Render(e *engine.Engine, frame renderer.RenderFrame) {
	// Draw tunnel (rotates with wand)
	tunnelModel := g.rotation
	tunnelMVP := frame.ViewProj.Mul4(tunnelModel)
	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: g.tunnel.VertexBuffer,
		IndexBuffer:  g.tunnel.IndexBuffer,
		IndexCount:   g.tunnel.IndexCount,
		MVP:          tunnelMVP,
		Model:        tunnelModel,
		NoFog:        true,
	})

	// Build and draw waveform
	g.renderWaveform(e, frame)
}

func (g *WandViewGame) renderWaveform(e *engine.Engine, frame renderer.RenderFrame) {
	// Release previous frame's buffers
	if g.waveVB != nil {
		e.Rend.ReleaseBuffer(g.waveVB)
		g.waveVB = nil
	}
	if g.waveIB != nil {
		e.Rend.ReleaseBuffer(g.waveIB)
		g.waveIB = nil
	}

	sampleCount := g.writePos
	if g.filled {
		sampleCount = numSamples
	}
	if sampleCount < 2 {
		return
	}

	const (
		waveZ      = float32(-3.0)
		waveXMin   = float32(-2.0)
		waveXMax   = float32(2.0)
		waveHeight = float32(0.8)
		thickness  = float32(0.03)
		maxAccel   = float32(25.0)
	)

	vertices := make([]renderer.LitVertex, 0, sampleCount*2)
	indices := make([]uint16, 0, (sampleCount-1)*6)

	for i := 0; i < sampleCount; i++ {
		// Read oldest-to-newest
		idx := i
		if g.filled {
			idx = (g.writePos + i) % numSamples
		}

		t := float32(i) / float32(sampleCount-1)
		x := waveXMin + t*(waveXMax-waveXMin)

		normalized := g.samples[idx] / maxAccel
		if normalized > 1 {
			normalized = 1
		}
		y := (normalized - 0.5) * 2.0 * waveHeight

		base := uint16(len(vertices))
		vertices = append(vertices,
			renderer.LitVertex{X: x, Y: y + thickness, Z: waveZ, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255},
			renderer.LitVertex{X: x, Y: y - thickness, Z: waveZ, NX: 0, NY: 0, NZ: 1, R: 255, G: 255, B: 255, A: 255},
		)

		if i > 0 {
			prev := base - 2
			indices = append(indices,
				prev, prev+1, base,
				base, prev+1, base+1,
			)
		}
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

	g.waveVB = vb
	g.waveIB = ib
	g.waveIdxCnt = uint32(len(indices))

	waveMVP := frame.ViewProj
	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: g.waveVB,
		IndexBuffer:  g.waveIB,
		IndexCount:   g.waveIdxCnt,
		MVP:          waveMVP,
		NoFog:        true,
	})
}

func (g *WandViewGame) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	if g.pause.IsActive() {
		g.pause.Render(e.Rend, cmdBuf, target, e.Win.Width(), e.Win.Height())
	}
}

func (g *WandViewGame) Destroy(e *engine.Engine) {
	g.wand.Stop()
	g.pause.Destroy(e.Rend)
	g.tunnel.Destroy(e.Rend)
	if g.waveVB != nil {
		e.Rend.ReleaseBuffer(g.waveVB)
	}
	if g.waveIB != nil {
		e.Rend.ReleaseBuffer(g.waveIB)
	}
}
