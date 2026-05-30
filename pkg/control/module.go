package control

import (
	"encoding/json"
	"net/http"
)

// GameModule is the web contribution of the currently-active game: its own
// control/settings page (Page, served under /game/) and REST (API, served under
// /api/game/). The request path passed to each is already stripped of that
// prefix, so a module sees "/settings", "/api/cards", etc.
//
// Implementations MUST be loop-thread-safe: handlers run on the HTTP goroutine
// concurrently with the game loop, so they may touch only their own
// mutex-guarded CPU state — never SDL, the GPU, or the engine. All GPU work
// belongs in the game loop, driven off a revision/snapshot diff.
type GameModule interface {
	Page(w http.ResponseWriter, r *http.Request)
	API(w http.ResponseWriter, r *http.Request)
}

// SettingType is the control kind the generic settings page renders.
type SettingType string

const SettingFloat SettingType = "float"

// Setting describes one tunable a game exposes to the generic settings page.
type Setting struct {
	Key   string      `json:"key"`
	Label string      `json:"label"`
	Type  SettingType `json:"type"`
	Min   float64     `json:"min"`
	Max   float64     `json:"max"`
	Value float64     `json:"value"`
}

// SettingsModel is implemented by a game's thread-safe tuning struct so the
// generic settings module can render controls and apply edits without the game
// writing any HTTP/HTML. Both methods are called from the HTTP goroutine and
// must lock internally.
type SettingsModel interface {
	Settings() []Setting
	Apply(key string, value float64)
}

// settingsModule adapts a SettingsModel into a GameModule: Page serves the
// generic slider SPA; API serves GET/POST /settings.
type settingsModule struct {
	title string
	model SettingsModel
}

// NewSettingsModule wraps a SettingsModel as a GameModule so a game with simple
// tunables gets a web page for free.
func NewSettingsModule(title string, model SettingsModel) GameModule {
	return &settingsModule{title: title, model: model}
}

type settingsDTO struct {
	Title    string    `json:"title"`
	Settings []Setting `json:"settings"`
}

func (m *settingsModule) Page(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(genericHTML)
}

func (m *settingsModule) API(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/settings" {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, settingsDTO{Title: m.title, Settings: m.model.Settings()})
	case http.MethodPost:
		var body struct {
			Key   string  `json:"key"`
			Value float64 `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		m.model.Apply(body.Key, body.Value)
		writeJSON(w, settingsDTO{Title: m.title, Settings: m.model.Settings()})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
