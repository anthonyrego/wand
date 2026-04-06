package input

import (
	"github.com/Zyko0/go-sdl3/sdl"
)

type Input struct {
	keyState     map[sdl.Keycode]bool
	prevKeyState map[sdl.Keycode]bool
	mouseX       float32
	mouseY       float32
	mouseDeltaX  float32
	mouseDeltaY  float32
	scrollY      float32
	quit         bool

	mouseLeftPressed bool
	mouseLeftDown    bool
	mouseRightDown   bool
	mouseClickX      float32
	mouseClickY      float32
}

func New() *Input {
	return &Input{
		keyState:     make(map[sdl.Keycode]bool),
		prevKeyState: make(map[sdl.Keycode]bool),
	}
}

func (i *Input) Update() {
	// Reset per-frame deltas
	i.mouseDeltaX = 0
	i.mouseDeltaY = 0
	i.scrollY = 0
	i.mouseLeftPressed = false

	// Copy current state to previous
	for k, v := range i.keyState {
		i.prevKeyState[k] = v
	}

	// Poll events
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_QUIT:
			i.quit = true

		case sdl.EVENT_KEY_DOWN:
			keyEvent := event.KeyboardEvent()
			i.keyState[keyEvent.Key] = true

		case sdl.EVENT_KEY_UP:
			keyEvent := event.KeyboardEvent()
			i.keyState[keyEvent.Key] = false

		case sdl.EVENT_MOUSE_MOTION:
			motionEvent := event.MouseMotionEvent()
			i.mouseX = motionEvent.X
			i.mouseY = motionEvent.Y
			i.mouseDeltaX = motionEvent.Xrel
			i.mouseDeltaY = motionEvent.Yrel

		case sdl.EVENT_MOUSE_BUTTON_DOWN:
			btn := event.MouseButtonEvent()
			if btn.Button == 1 { // left
				i.mouseLeftPressed = true
				i.mouseLeftDown = true
				i.mouseClickX = btn.X
				i.mouseClickY = btn.Y
			}
			if btn.Button == 3 { // right
				i.mouseRightDown = true
			}

		case sdl.EVENT_MOUSE_BUTTON_UP:
			btn := event.MouseButtonEvent()
			if btn.Button == 1 { // left
				i.mouseLeftDown = false
			}
			if btn.Button == 3 { // right
				i.mouseRightDown = false
			}

		case sdl.EVENT_MOUSE_WHEEL:
			wheelEvent := event.MouseWheelEvent()
			i.scrollY += wheelEvent.Y
		}
	}
}

func (i *Input) IsKeyDown(key sdl.Keycode) bool {
	return i.keyState[key]
}

func (i *Input) IsKeyPressed(key sdl.Keycode) bool {
	return i.keyState[key] && !i.prevKeyState[key]
}

func (i *Input) IsKeyReleased(key sdl.Keycode) bool {
	return !i.keyState[key] && i.prevKeyState[key]
}

func (i *Input) MousePosition() (float32, float32) {
	return i.mouseX, i.mouseY
}

func (i *Input) MouseDelta() (float32, float32) {
	return i.mouseDeltaX, i.mouseDeltaY
}

func (i *Input) ScrollY() float32 {
	return i.scrollY
}

func (i *Input) IsMouseLeftPressed() bool {
	return i.mouseLeftPressed
}

func (i *Input) IsMouseLeftDown() bool {
	return i.mouseLeftDown
}

func (i *Input) IsMouseRightDown() bool {
	return i.mouseRightDown
}

func (i *Input) MouseClickPosition() (float32, float32) {
	return i.mouseClickX, i.mouseClickY
}

func (i *Input) ShouldQuit() bool {
	return i.quit
}
