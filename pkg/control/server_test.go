package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthonyrego/toybox/pkg/control/profile"
	"github.com/anthonyrego/toybox/pkg/settings"
)

func newTestServer(t *testing.T) (*httptest.Server, Deps) {
	t.Helper()
	name, err := profile.Load(t.TempDir())
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	deps := Deps{
		Name:  name,
		Video: NewVideoStore(""), // in-memory only
		Nav:   NewNavStore([]GameInfo{{ID: "drum-circle", Name: "DRUM CIRCLE"}}),
	}
	srv := httptest.NewServer(NewServer(deps, 0).Handler())
	t.Cleanup(srv.Close)
	return srv, deps
}

func TestProfileGetSet(t *testing.T) {
	srv, deps := newTestServer(t)

	resp, err := http.Post(srv.URL+"/api/profile", "application/json",
		strings.NewReader(`{"name":"Emma"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var set profileDTO
	_ = json.NewDecoder(resp.Body).Decode(&set)
	resp.Body.Close()
	if set.Name != "Emma" || set.Title != "EMMA'S TOY BOX" {
		t.Fatalf("POST result: %+v", set)
	}
	if deps.Name.Name() != "Emma" {
		t.Fatalf("store not updated: %q", deps.Name.Name())
	}
}

func TestGamesAndCurrent(t *testing.T) {
	srv, deps := newTestServer(t)

	// Games list.
	resp, err := http.Get(srv.URL + "/api/games")
	if err != nil {
		t.Fatalf("GET games: %v", err)
	}
	var games []GameInfo
	_ = json.NewDecoder(resp.Body).Decode(&games)
	resp.Body.Close()
	if len(games) != 1 || games[0].ID != "drum-circle" {
		t.Fatalf("games: %+v", games)
	}

	// Request a valid switch -> 204 and a pending request the App can take.
	resp, err = http.Post(srv.URL+"/api/current", "application/json", strings.NewReader(`{"id":"drum-circle"}`))
	if err != nil {
		t.Fatalf("POST current: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST current status = %d", resp.StatusCode)
	}
	if id, ok := deps.Nav.TakeRequest(); !ok || id != "drum-circle" {
		t.Fatalf("TakeRequest = %q,%v", id, ok)
	}

	// Unknown id -> 400.
	resp, err = http.Post(srv.URL+"/api/current", "application/json", strings.NewReader(`{"id":"nope"}`))
	if err != nil {
		t.Fatalf("POST bad current: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad id status = %d", resp.StatusCode)
	}
}

func TestVideoGetSetPersistsToStore(t *testing.T) {
	srv, deps := newTestServer(t)

	resp, err := http.Post(srv.URL+"/api/video", "application/json",
		strings.NewReader(`{"windowWidth":1920,"windowHeight":1080,"fullscreen":true,"renderDistance":1000}`))
	if err != nil {
		t.Fatalf("POST video: %v", err)
	}
	var got settings.Settings
	_ = json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if got.WindowWidth != 1920 || !got.Fullscreen || got.RenderDistance != 1000 {
		t.Fatalf("video result: %+v", got)
	}
	if deps.Video.Settings().WindowWidth != 1920 {
		t.Fatalf("store not updated: %+v", deps.Video.Settings())
	}
}

func TestVideoModes(t *testing.T) {
	name, err := profile.Load(t.TempDir())
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	deps := Deps{
		Name:        name,
		Video:       NewVideoStore(""),
		Nav:         NewNavStore(nil),
		Resolutions: []Resolution{{W: 1920, H: 1080}, {W: 1280, H: 720}},
	}
	ts := httptest.NewServer(NewServer(deps, 0).Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/video/modes")
	if err != nil {
		t.Fatalf("GET modes: %v", err)
	}
	var got []Resolution
	_ = json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if len(got) != 2 || got[0].W != 1920 || got[1].H != 720 {
		t.Fatalf("modes: %+v", got)
	}
}

func TestActiveModuleDispatch(t *testing.T) {
	srv, _ := newTestServer(t)

	// No active module yet -> 503.
	resp, err := http.Get(srv.URL + "/game/")
	if err != nil {
		t.Fatalf("GET game: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("inactive /game/ status = %d", resp.StatusCode)
	}
}

// fakeModule records the (already prefix-stripped) path each handler sees.
type fakeModule struct{ pagePath, apiPath string }

func (f *fakeModule) Page(w http.ResponseWriter, r *http.Request) { f.pagePath = r.URL.Path }
func (f *fakeModule) API(w http.ResponseWriter, r *http.Request)  { f.apiPath = r.URL.Path }

func TestActiveModulePrefixStripping(t *testing.T) {
	name, err := profile.Load(t.TempDir())
	if err != nil {
		t.Fatalf("profile.Load: %v", err)
	}
	srv := NewServer(Deps{Name: name, Video: NewVideoStore(""), Nav: NewNavStore(nil)}, 0)
	fm := &fakeModule{}
	srv.SetActiveModule(fm)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	if _, err := http.Get(ts.URL + "/game/"); err != nil {
		t.Fatalf("GET /game/: %v", err)
	}
	if fm.pagePath != "/" {
		t.Fatalf("Page saw path %q, want %q", fm.pagePath, "/")
	}

	if _, err := http.Get(ts.URL + "/api/game/settings"); err != nil {
		t.Fatalf("GET /api/game/settings: %v", err)
	}
	if fm.apiPath != "/settings" {
		t.Fatalf("API saw path %q, want %q", fm.apiPath, "/settings")
	}
}
