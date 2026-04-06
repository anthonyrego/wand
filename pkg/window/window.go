package window

import (
	"errors"
	"sort"

	"github.com/Zyko0/go-sdl3/sdl"
)

type Resolution struct {
	W, H int
}

type Window struct {
	handle         *sdl.Window
	device         *sdl.GPUDevice
	width          int
	height         int
	title          string
	fullscreen     bool
	windowedWidth  int
	windowedHeight int
}

type Config struct {
	Title  string
	Width  int
	Height int
}

func New(cfg Config) (*Window, error) {
	device, err := sdl.CreateGPUDevice(
		sdl.GPU_SHADERFORMAT_SPIRV|sdl.GPU_SHADERFORMAT_DXIL|sdl.GPU_SHADERFORMAT_MSL,
		true,
		"",
	)
	if err != nil {
		return nil, errors.New("failed to create GPU device: " + err.Error())
	}

	handle, err := sdl.CreateWindow(cfg.Title, cfg.Width, cfg.Height, sdl.WINDOW_RESIZABLE)
	if err != nil {
		device.Destroy()
		return nil, errors.New("failed to create window: " + err.Error())
	}

	err = device.ClaimWindow(handle)
	if err != nil {
		handle.Destroy()
		device.Destroy()
		return nil, errors.New("failed to claim window: " + err.Error())
	}

	return &Window{
		handle:         handle,
		device:         device,
		width:          cfg.Width,
		height:         cfg.Height,
		title:          cfg.Title,
		windowedWidth:  cfg.Width,
		windowedHeight: cfg.Height,
	}, nil
}

func (w *Window) Handle() *sdl.Window {
	return w.handle
}

func (w *Window) Device() *sdl.GPUDevice {
	return w.device
}

func (w *Window) Width() int {
	return w.width
}

func (w *Window) Height() int {
	return w.height
}

func (w *Window) IsFullscreen() bool {
	return w.fullscreen
}

func (w *Window) SetFullscreen(fullscreen bool) error {
	if fullscreen == w.fullscreen {
		return nil
	}
	if fullscreen {
		// Save windowed size before entering fullscreen
		w.windowedWidth = w.width
		w.windowedHeight = w.height
	}
	if err := w.handle.SetFullscreen(fullscreen); err != nil {
		return err
	}
	w.fullscreen = fullscreen

	if fullscreen {
		// Use the display's native resolution
		displayID := sdl.GetDisplayForWindow(w.handle)
		mode, err := displayID.DesktopDisplayMode()
		if err == nil {
			w.width = int(mode.W)
			w.height = int(mode.H)
		}
	} else {
		// Restore windowed size
		w.handle.SetSize(int32(w.windowedWidth), int32(w.windowedHeight))
		w.width = w.windowedWidth
		w.height = w.windowedHeight
	}
	return nil
}

func (w *Window) SetSize(width, height int) error {
	if w.fullscreen {
		return nil
	}
	if err := w.handle.SetSize(int32(width), int32(height)); err != nil {
		return err
	}
	w.width = width
	w.height = height
	w.windowedWidth = width
	w.windowedHeight = height
	return nil
}

func (w *Window) DisplayModes() []Resolution {
	displayID := sdl.GetDisplayForWindow(w.handle)
	modes, err := displayID.FullscreenDisplayModes()
	if err != nil || len(modes) == 0 {
		return []Resolution{
			{1280, 720},
			{1920, 1080},
		}
	}

	// Get desktop mode to determine max usable window size
	maxW, maxH := 3840, 2160
	if desktop, err := displayID.DesktopDisplayMode(); err == nil {
		maxW = int(desktop.W)
		maxH = int(desktop.H)
		// Account for window chrome (title bar, menu bar, dock)
		maxH -= 80
	}

	// Deduplicate by (W, H), filter to fit within desktop
	seen := make(map[[2]int]bool)
	var result []Resolution
	for _, m := range modes {
		mw, mh := int(m.W), int(m.H)
		key := [2]int{mw, mh}
		if seen[key] || mw < 640 || mh < 480 || mw > maxW || mh > maxH {
			continue
		}
		seen[key] = true
		result = append(result, Resolution{W: mw, H: mh})
	}

	// Sort descending by pixel count
	sort.Slice(result, func(i, j int) bool {
		return result[i].W*result[i].H > result[j].W*result[j].H
	})

	if len(result) == 0 {
		return []Resolution{{1280, 720}}
	}
	return result
}

func (w *Window) SetRelativeMouseMode(enabled bool) error {
	return w.handle.SetRelativeMouseMode(enabled)
}

func (w *Window) Destroy() {
	w.device.ReleaseWindow(w.handle)
	w.handle.Destroy()
	w.device.Destroy()
}
