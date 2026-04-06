package engine

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/wand/pkg/camera"
	"github.com/anthonyrego/wand/pkg/input"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/settings"
	"github.com/anthonyrego/wand/pkg/window"
)

// Game is implemented by the application to provide game-specific logic.
type Game interface {
	Init(e *Engine) error
	Update(e *Engine, dt float32) bool // returns false to quit
	Render(e *Engine, frame renderer.RenderFrame)
	Overlay(e *Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture)
	Destroy(e *Engine)
}

// Engine owns the window, renderer, camera, input, and rendering lifecycle.
type Engine struct {
	Win   *window.Window
	Rend  *renderer.Renderer
	Cam   *camera.Camera
	Input *input.Input

	PixelScale int

	// Set by game in Update, used by engine during render.
	LightUniforms renderer.LightUniforms
	PostProcess   renderer.PostProcessUniforms
}

// New creates an Engine with a window, renderer, camera, and input handler.
func New(title string, ds settings.Settings) (*Engine, error) {
	win, err := window.New(window.Config{
		Title:  title,
		Width:  ds.WindowWidth,
		Height: ds.WindowHeight,
	})
	if err != nil {
		return nil, fmt.Errorf("window: %w", err)
	}

	if ds.Fullscreen {
		if err := win.SetFullscreen(true); err != nil {
			fmt.Println("Warning: could not set fullscreen:", err)
		}
	}

	if err := win.SetRelativeMouseMode(true); err != nil {
		fmt.Println("Warning: could not enable relative mouse mode:", err)
	}

	rend, err := renderer.New(win)
	if err != nil {
		win.Destroy()
		return nil, fmt.Errorf("renderer: %w", err)
	}

	pixelScale := ds.PixelScale
	offW := uint32(win.Width() / pixelScale)
	offH := uint32(win.Height() / pixelScale)
	if offW < 1 {
		offW = 1
	}
	if offH < 1 {
		offH = 1
	}
	rend.SetOffscreenResolution(offW, offH)

	cam := camera.New(float32(win.Width()) / float32(win.Height()))
	cam.Far = ds.RenderDistance

	return &Engine{
		Win:        win,
		Rend:       rend,
		Cam:        cam,
		Input:      input.New(),
		PixelScale: pixelScale,
	}, nil
}

// Run executes the game loop until the game returns false from Update or SDL quits.
func (e *Engine) Run(game Game) error {
	if err := game.Init(e); err != nil {
		return fmt.Errorf("game init: %w", err)
	}
	defer game.Destroy(e)

	lastTime := time.Now()

	for {
		currentTime := time.Now()
		dt := float32(currentTime.Sub(lastTime).Seconds())
		lastTime = currentTime

		e.Input.Update()

		if e.Input.ShouldQuit() {
			break
		}

		if !game.Update(e, dt) {
			break
		}

		// Stamp camera position into light uniforms (headlamp + fog)
		e.LightUniforms.CameraPos = mgl32.Vec4{
			e.Cam.Position.X(), e.Cam.Position.Y(), e.Cam.Position.Z(), 0,
		}
		e.LightUniforms.LightPositions[0] = e.LightUniforms.CameraPos
		e.LightUniforms.FogParams[2] = e.Cam.Far

		// --- Two-pass rendering ---

		// Pass 1: Scene to offscreen texture
		cmdBuf, err := e.Rend.BeginLitFrame()
		if err != nil {
			fmt.Println("Error beginning lit frame:", err)
			continue
		}

		scenePass := e.Rend.BeginScenePass(cmdBuf)
		e.Rend.PushLightUniforms(cmdBuf, e.LightUniforms)

		viewProj := e.Cam.ViewProjectionMatrix()
		frustum := camera.ExtractFrustum(viewProj)
		cullDist := e.Cam.Far * 0.90
		cullDistSq := cullDist * cullDist
		fadeStart := cullDist * 0.80
		fadeRange := cullDist - fadeStart

		frame := renderer.RenderFrame{
			CmdBuf:     cmdBuf,
			ScenePass:  scenePass,
			ViewProj:   viewProj,
			Frustum:    frustum,
			CamPos:     e.Cam.Position,
			CamRight:   e.Cam.Right(),
			CamUp:      e.Cam.Up(),
			CullDist:   cullDist,
			CullDistSq: cullDistSq,
			FadeStart:  fadeStart,
			FadeRange:  fadeRange,
		}

		game.Render(e, frame)

		e.Rend.EndScenePass(scenePass)

		// Pass 2: Post-process + overlays
		swapchain, err := cmdBuf.WaitAndAcquireGPUSwapchainTexture(e.Win.Handle())
		if err != nil {
			fmt.Println("Error acquiring swapchain:", err)
			e.Rend.EndLitFrame(cmdBuf)
			continue
		}

		if swapchain != nil {
			e.Rend.RunPostProcess(cmdBuf, swapchain.Texture, e.PostProcess)
			game.Overlay(e, cmdBuf, swapchain.Texture)
		}

		e.Rend.EndLitFrame(cmdBuf)
	}

	return nil
}

// ApplyDisplaySettings updates window, renderer, and camera for new display settings.
func (e *Engine) ApplyDisplaySettings(fullscreen bool, w, h, pixelScale int, renderDistance float32) {
	e.PixelScale = pixelScale
	e.Cam.Far = renderDistance

	if err := e.Win.SetFullscreen(fullscreen); err != nil {
		fmt.Println("Fullscreen error:", err)
	}
	if !fullscreen {
		e.Win.SetSize(w, h)
	}
	e.Cam.AspectRatio = float32(e.Win.Width()) / float32(e.Win.Height())

	newOffW := uint32(e.Win.Width() / pixelScale)
	newOffH := uint32(e.Win.Height() / pixelScale)
	if newOffW < 1 {
		newOffW = 1
	}
	if newOffH < 1 {
		newOffH = 1
	}
	e.Rend.SetOffscreenResolution(newOffW, newOffH)
}

// SetMouseMode toggles between relative (FPS) and absolute (menu) mouse mode.
func (e *Engine) SetMouseMode(relative bool) {
	e.Win.SetRelativeMouseMode(relative)
}

// Destroy releases the renderer and window.
func (e *Engine) Destroy() {
	e.Rend.Destroy()
	e.Win.Destroy()
}
