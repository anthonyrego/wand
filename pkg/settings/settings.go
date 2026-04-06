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
	PixelScale     int     `json:"pixelScale"`
	RenderDistance float32 `json:"renderDistance"`
}

func Default() Settings {
	return Settings{
		WindowWidth:    1280,
		WindowHeight:   720,
		Fullscreen:     false,
		PixelScale:     4,
		RenderDistance:  1500,
	}
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
	if s.WindowWidth <= 0 || s.WindowHeight <= 0 {
		d := Default()
		s.WindowWidth = d.WindowWidth
		s.WindowHeight = d.WindowHeight
	}
	if s.PixelScale <= 0 {
		s.PixelScale = Default().PixelScale
	}
	if s.RenderDistance <= 0 {
		s.RenderDistance = Default().RenderDistance
	}
	return s
}

func Save(path string, s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
