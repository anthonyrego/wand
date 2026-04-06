package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SceneConfig holds hot-reloadable rendering and environment configuration.
// Deserialized from scene.json.
type SceneConfig struct {
	PostProcess struct {
		DitherStrength float32 `json:"ditherStrength"`
		ColorLevels    float32 `json:"colorLevels"`
		TintR          float32 `json:"tintR"`
		TintG          float32 `json:"tintG"`
		TintB          float32 `json:"tintB"`
	} `json:"postProcess"`
	Lighting struct {
		AmbientR             float32 `json:"ambientR"`
		AmbientG             float32 `json:"ambientG"`
		AmbientB             float32 `json:"ambientB"`
		StreetLightR         float32 `json:"streetLightR"`
		StreetLightG         float32 `json:"streetLightG"`
		StreetLightB         float32 `json:"streetLightB"`
		StreetLightIntensity float32 `json:"streetLightIntensity"`
		SunDirX              float32 `json:"sunDirX"`
		SunDirY              float32 `json:"sunDirY"`
		SunDirZ              float32 `json:"sunDirZ"`
		SunR                 float32 `json:"sunR"`
		SunG                 float32 `json:"sunG"`
		SunB                 float32 `json:"sunB"`
		SunIntensity         float32 `json:"sunIntensity"`
	} `json:"lighting"`
	Headlamp struct {
		R         float32 `json:"r"`
		G         float32 `json:"g"`
		B         float32 `json:"b"`
		Intensity float32 `json:"intensity"`
	} `json:"headlamp"`
	Snow struct {
		Count        int     `json:"count"`
		FallSpeed    float32 `json:"fallSpeed"`
		WindStrength float32 `json:"windStrength"`
		ParticleSize float32 `json:"particleSize"`
	} `json:"snow"`
	Fog struct {
		R     float32 `json:"r"`
		G     float32 `json:"g"`
		B     float32 `json:"b"`
		Start float32 `json:"start"`
		End   float32 `json:"end"`
	} `json:"fog"`
	Textures struct {
		GroundScale    float32 `json:"groundScale"`
		GroundStrength float32 `json:"groundStrength"`
		MaterialScale  float32 `json:"materialScale"`
	} `json:"textures"`
	Sky struct {
		Speed     float32 `json:"speed"`
		Scale     float32 `json:"scale"`
		Intensity float32 `json:"intensity"`
	} `json:"sky"`
}

// ConfigWatcher polls a JSON config file for changes.
type ConfigWatcher struct {
	path    string
	modTime time.Time
}

// NewConfigWatcher creates a watcher for the given config file path.
func NewConfigWatcher(path string) *ConfigWatcher {
	return &ConfigWatcher{path: path}
}

// Check returns the parsed config if the file has been modified since the last check.
func (cw *ConfigWatcher) Check() (*SceneConfig, bool) {
	info, err := os.Stat(cw.path)
	if err != nil {
		return nil, false
	}
	if !info.ModTime().After(cw.modTime) {
		return nil, false
	}
	cw.modTime = info.ModTime()

	data, err := os.ReadFile(cw.path)
	if err != nil {
		fmt.Println("Config read error:", err)
		return nil, false
	}

	var cfg SceneConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Println("Config parse error:", err)
		return nil, false
	}

	fmt.Println("Config reloaded")
	return &cfg, true
}

// ForceReload resets the modification time so the next Check triggers a reload.
func (cw *ConfigWatcher) ForceReload() {
	cw.modTime = time.Time{}
}
