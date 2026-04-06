package games

import (
	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/anthonyrego/wand"
	"github.com/anthonyrego/wand/games/selector"
	"github.com/anthonyrego/wand/pkg/engine"
	"github.com/anthonyrego/wand/pkg/renderer"
)

const switchToSelector = -2

type App struct {
	wand     *wand.Listener
	defs     []GameDef
	names    []string
	selector *selector.Selector
	current  engine.Game
	switchTo int // -1 = none, -2 = selector, >=0 = game index
}

func NewApp(w *wand.Listener, defs []GameDef) *App {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return &App{
		wand:     w,
		defs:     defs,
		names:    names,
		switchTo: -1,
	}
}

func (a *App) Init(e *engine.Engine) error {
	a.selector = selector.New(a.names)
	a.current = a.selector
	return a.current.Init(e)
}

func (a *App) Update(e *engine.Engine, dt float32) bool {
	// Handle pending game switch
	if a.switchTo != -1 {
		a.current.Destroy(e)
		if a.switchTo == switchToSelector {
			a.selector = selector.New(a.names)
			a.current = a.selector
		} else {
			a.current = a.defs[a.switchTo].New(a.wand)
		}
		a.switchTo = -1
		if err := a.current.Init(e); err != nil {
			return false
		}
	}

	if !a.current.Update(e, dt) {
		return false
	}

	// Check if selector made a choice
	if sel, ok := a.current.(*selector.Selector); ok {
		if idx := sel.Chosen(); idx >= 0 {
			a.switchTo = idx
		}
	}

	// Check if active game wants to return to selector
	if gs, ok := a.current.(GameSwitcher); ok {
		if gs.WantsChangeGame() {
			a.switchTo = switchToSelector
		}
	}

	return true
}

func (a *App) Render(e *engine.Engine, frame renderer.RenderFrame) {
	a.current.Render(e, frame)
}

func (a *App) Overlay(e *engine.Engine, cmdBuf *sdl.GPUCommandBuffer, target *sdl.GPUTexture) {
	a.current.Overlay(e, cmdBuf, target)
}

func (a *App) Destroy(e *engine.Engine) {
	a.current.Destroy(e)
}
