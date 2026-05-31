// Package calibration provides a small, shared "point the wand straight up and
// press Calibrate" helper for every game that consumes wand orientation.
//
// Instead of implicitly capturing a neutral pose on the first frame, a wand game
// embeds a Calibrator and waits for the player to press a Calibrate button on
// the web control page. The web (HTTP goroutine) calls Request; the game loop
// calls Track once per frame, which folds any pending request in on the loop
// thread so the captured neutral quaternion is only ever written by a single
// owner. Until the first calibration Track returns identity, so the game sits at
// its neutral pose while the on-screen prompt is shown.
package calibration

import (
	"sync"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/anthonyrego/toybox/pkg/control"
)

// CalibrateKey is the Setting key used for the Calibrate button on a game's web
// page. A button press arrives as control.SettingsModel.Apply(CalibrateKey, _).
const CalibrateKey = "calibrate"

var (
	// wandRemap re-expresses the wand's reported orientation in the engine's
	// camera-aligned frame (forward −Z, up +Y, right +X). The IMU is mounted
	// rotated inside the wand: measured calibrated deltas put a physical pitch on
	// the IMU +X axis, roll on −Y, yaw on −Z. A +90° rotation about X maps those
	// onto the camera's pitch(+X)/roll(−Z)/yaw(+Y) axes so a wand tilt reads as
	// the same tilt on screen. Applied as a similarity transform R·q·R⁻¹. If you
	// ever bake this into the firmware WAND_REMAP, reset this to identity.
	wandRemap    = mgl32.Quat{W: 0.70710677, V: mgl32.Vec3{0.70710677, 0, 0}}
	wandRemapInv = wandRemap.Conjugate()
)

// Calibrator captures a neutral reference orientation on demand. The zero value
// is not usable; create one with New.
type Calibrator struct {
	mu         sync.Mutex
	pending    bool       // a calibration was requested from the web
	calibrated bool       // a neutral has been captured at least once
	neutral    mgl32.Quat // the captured reference orientation
}

// New returns a Calibrator seeded with an identity neutral.
func New() *Calibrator {
	return &Calibrator{neutral: mgl32.QuatIdent()}
}

// Request marks that the next loop frame should capture the current orientation
// as the neutral reference. Safe to call from the HTTP goroutine.
func (c *Calibrator) Request() {
	c.mu.Lock()
	c.pending = true
	c.mu.Unlock()
}

// Track folds a pending calibration request in using the current wand
// orientation and returns the body-frame delta from the captured neutral to
// current (neutral.Inverse() * current). Call once per frame from the game loop.
// Before the first calibration it returns identity so the game holds still.
func (c *Calibrator) Track(current mgl32.Quat) mgl32.Quat {
	current = wandRemap.Mul(current).Mul(wandRemapInv)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pending {
		c.pending = false
		c.neutral = current
		c.calibrated = true
	}
	if !c.calibrated {
		return mgl32.QuatIdent()
	}
	return c.neutral.Inverse().Mul(current)
}

// Calibrated reports whether a neutral reference has been captured yet. While it
// is false the App draws the "hold the wand up and press Calibrate" prompt.
func (c *Calibrator) Calibrated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calibrated
}

// Setting is the descriptor a game adds to its web settings to render the
// Calibrate button.
func Setting() control.Setting {
	return control.Setting{
		Key:   CalibrateKey,
		Label: "Calibrate",
		Type:  control.SettingButton,
	}
}

// Settings implements control.SettingsModel for a game whose only web control is
// the Calibrate button (e.g. Color Sphere, Flying). Games with their own sliders
// use WithCalibrate instead.
func (c *Calibrator) Settings() []control.Setting {
	return []control.Setting{Setting()}
}

// Apply implements control.SettingsModel — a press of the Calibrate button.
func (c *Calibrator) Apply(key string, value float64) {
	if key == CalibrateKey {
		c.Request()
	}
}

// WithCalibrate returns a control.SettingsModel that exposes model's settings
// plus a trailing Calibrate button, routing that button's presses to c. Use it
// for games that already have their own sliders (e.g. Drum Circle).
func WithCalibrate(model control.SettingsModel, c *Calibrator) control.SettingsModel {
	return combined{model: model, cal: c}
}

type combined struct {
	model control.SettingsModel
	cal   *Calibrator
}

func (m combined) Settings() []control.Setting {
	return append(m.model.Settings(), Setting())
}

func (m combined) Apply(key string, value float64) {
	if key == CalibrateKey {
		m.cal.Request()
		return
	}
	m.model.Apply(key, value)
}
