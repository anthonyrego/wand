package main

import (
	"fmt"
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

type WandViewGame struct {
	wand     *wand.Listener
	cube     *mesh.Mesh
	pause    *ui.PauseMenu
	rotation mgl32.Mat4
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

	e.Cam.Far = 100

	// Multi-colored cube — same geometry as mesh.NewLitCube but with distinct face colors
	vertices := []renderer.LitVertex{
		// Front face (Z+) — Red
		{X: -0.5, Y: -0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: 220, G: 50, B: 50, A: 255},
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: 220, G: 50, B: 50, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: 220, G: 50, B: 50, A: 255},
		{X: -0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: 220, G: 50, B: 50, A: 255},
		// Back face (Z-) — Green
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: 50, G: 180, B: 50, A: 255},
		{X: -0.5, Y: -0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: 50, G: 180, B: 50, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: 50, G: 180, B: 50, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: 50, G: 180, B: 50, A: 255},
		// Top face (Y+) — Blue
		{X: -0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 1, NZ: 0, R: 50, G: 100, B: 220, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 1, NZ: 0, R: 50, G: 100, B: 220, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 1, NZ: 0, R: 50, G: 100, B: 220, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 1, NZ: 0, R: 50, G: 100, B: 220, A: 255},
		// Bottom face (Y-) — Yellow
		{X: -0.5, Y: -0.5, Z: -0.5, NX: 0, NY: -1, NZ: 0, R: 220, G: 200, B: 50, A: 255},
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 0, NY: -1, NZ: 0, R: 220, G: 200, B: 50, A: 255},
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 0, NY: -1, NZ: 0, R: 220, G: 200, B: 50, A: 255},
		{X: -0.5, Y: -0.5, Z: 0.5, NX: 0, NY: -1, NZ: 0, R: 220, G: 200, B: 50, A: 255},
		// Right face (X+) — Orange
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 1, NY: 0, NZ: 0, R: 220, G: 130, B: 30, A: 255},
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 1, NY: 0, NZ: 0, R: 220, G: 130, B: 30, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 1, NY: 0, NZ: 0, R: 220, G: 130, B: 30, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 1, NY: 0, NZ: 0, R: 220, G: 130, B: 30, A: 255},
		// Left face (X-) — Purple
		{X: -0.5, Y: -0.5, Z: -0.5, NX: -1, NY: 0, NZ: 0, R: 160, G: 50, B: 200, A: 255},
		{X: -0.5, Y: -0.5, Z: 0.5, NX: -1, NY: 0, NZ: 0, R: 160, G: 50, B: 200, A: 255},
		{X: -0.5, Y: 0.5, Z: 0.5, NX: -1, NY: 0, NZ: 0, R: 160, G: 50, B: 200, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: -1, NY: 0, NZ: 0, R: 160, G: 50, B: 200, A: 255},
	}

	indices := []uint16{
		0, 1, 2, 0, 2, 3, // Front
		4, 5, 6, 4, 6, 7, // Back
		8, 9, 10, 8, 10, 11, // Top
		12, 13, 14, 12, 14, 15, // Bottom
		16, 17, 18, 16, 18, 19, // Right
		20, 21, 22, 20, 22, 23, // Left
	}

	vb, err := e.Rend.CreateLitVertexBuffer(vertices)
	if err != nil {
		return fmt.Errorf("cube vertex buffer: %w", err)
	}
	ib, err := e.Rend.CreateIndexBuffer(indices)
	if err != nil {
		return fmt.Errorf("cube index buffer: %w", err)
	}
	g.cube = &mesh.Mesh{VertexBuffer: vb, IndexBuffer: ib, IndexCount: uint32(len(indices))}

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

	// Lighting
	e.LightUniforms.AmbientColor = mgl32.Vec4{0.3, 0.3, 0.3, 1.0}
	e.LightUniforms.SunDirection = mgl32.Vec4{0.5, 0.8, 0.3, 0}
	e.LightUniforms.SunColor = mgl32.Vec4{1.0, 1.0, 0.95, 0.7}

	// Post-process
	e.PostProcess = renderer.PostProcessUniforms{
		Dither: mgl32.Vec4{0.5, 16.0, 0, 0},
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
	}

	return true
}

func (g *WandViewGame) Render(e *engine.Engine, frame renderer.RenderFrame) {
	model := g.rotation
	mvp := frame.ViewProj.Mul4(model)

	e.Rend.DrawLit(frame.CmdBuf, frame.ScenePass, renderer.LitDrawCall{
		VertexBuffer: g.cube.VertexBuffer,
		IndexBuffer:  g.cube.IndexBuffer,
		IndexCount:   g.cube.IndexCount,
		MVP:          mvp,
		Model:        model,
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
	g.cube.Destroy(e.Rend)
}
