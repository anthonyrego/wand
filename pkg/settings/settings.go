package settings

import (
	"encoding/json"
	"fmt"
	"os"
)

type Settings struct {
	WindowWidth    int     `json:"windowWidth"`
	WindowHeight   int     `json:"windowHeight"`
	Fullscreen     bool    `json:"fullscreen"`
	RenderDistance float32 `json:"renderDistance"`
}

func Default() Settings {
	return Settings{
		WindowWidth:    1280,
		WindowHeight:   720,
		Fullscreen:     false,
		RenderDistance: 1500,
	}
}

// Sanitize fills in sane defaults for any out-of-range field, so a partially
// written or hand-edited settings file can never produce a broken display.
func Sanitize(s Settings) Settings {
	d := Default()
	if s.WindowWidth <= 0 || s.WindowHeight <= 0 {
		s.WindowWidth = d.WindowWidth
		s.WindowHeight = d.WindowHeight
	}
	if s.RenderDistance <= 0 {
		s.RenderDistance = d.RenderDistance
	}
	return s
}

func Load(path string) Settings {
	data, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Println("Settings parse error:", err)
		return Default()
	}
	return Sanitize(s)
}

func Save(path string, s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
