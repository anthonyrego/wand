package drumcircle

import (
	"sync"

	"github.com/anthonyrego/wand/pkg/control"
)

// tuning holds the drum-circle hit/trail parameters. It is shared between the
// game loop (which reads a snapshot each frame) and the control server's HTTP
// goroutine (which mutates it via Apply). All access is guarded by mu; the loop
// never touches GPU state from these, so a plain lock is sufficient.
type tuning struct {
	mu             sync.Mutex
	hitThreshold   float32 // accel magnitude to register a hit
	accelCooldown  float32 // seconds between hits
	maxAccel       float32 // accel magnitude treated as a full-intensity hit
	visualExponent float32 // response curve for the visual burst
	audioExponent  float32 // response curve for the tone amplitude
	gyroThreshold  float32 // °/s to start emitting the motion trail
}

// tuningVals is a lock-free copy the game loop reads each frame.
type tuningVals struct {
	hitThreshold, accelCooldown, maxAccel, visualExponent, audioExponent, gyroThreshold float32
}

func (t *tuning) set(hit, cooldown, maxA, vis, aud, gyro float32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hitThreshold = hit
	t.accelCooldown = cooldown
	t.maxAccel = maxA
	t.visualExponent = vis
	t.audioExponent = aud
	t.gyroThreshold = gyro
}

func (t *tuning) snapshot() tuningVals {
	t.mu.Lock()
	defer t.mu.Unlock()
	return tuningVals{t.hitThreshold, t.accelCooldown, t.maxAccel, t.visualExponent, t.audioExponent, t.gyroThreshold}
}

// Settings implements control.SettingsModel — the descriptor the generic web
// page renders as sliders.
func (t *tuning) Settings() []control.Setting {
	t.mu.Lock()
	defer t.mu.Unlock()
	return []control.Setting{
		{Key: "hitThreshold", Label: "Hit threshold", Type: control.SettingFloat, Min: 2, Max: 15, Value: float64(t.hitThreshold)},
		{Key: "maxAccel", Label: "Full-hit accel", Type: control.SettingFloat, Min: 8, Max: 30, Value: float64(t.maxAccel)},
		{Key: "accelCooldown", Label: "Hit cooldown (s)", Type: control.SettingFloat, Min: 0.05, Max: 0.5, Value: float64(t.accelCooldown)},
		{Key: "visualExponent", Label: "Visual curve", Type: control.SettingFloat, Min: 0.5, Max: 5, Value: float64(t.visualExponent)},
		{Key: "audioExponent", Label: "Audio curve", Type: control.SettingFloat, Min: 0.2, Max: 3, Value: float64(t.audioExponent)},
		{Key: "gyroThreshold", Label: "Trail threshold", Type: control.SettingFloat, Min: 5, Max: 80, Value: float64(t.gyroThreshold)},
	}
}

// Apply implements control.SettingsModel — one slider edit from the web.
func (t *tuning) Apply(key string, value float64) {
	v := float32(value)
	t.mu.Lock()
	defer t.mu.Unlock()
	switch key {
	case "hitThreshold":
		t.hitThreshold = clampf(v, 2, 15)
	case "maxAccel":
		t.maxAccel = clampf(v, 8, 30)
	case "accelCooldown":
		t.accelCooldown = clampf(v, 0.05, 0.5)
	case "visualExponent":
		t.visualExponent = clampf(v, 0.5, 5)
	case "audioExponent":
		t.audioExponent = clampf(v, 0.2, 3)
	case "gyroThreshold":
		t.gyroThreshold = clampf(v, 5, 80)
	}
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
