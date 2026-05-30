package flashcard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthonyrego/toybox/pkg/control"
	"github.com/anthonyrego/toybox/pkg/control/profile"
)

// TestModuleUnderControlServer exercises the flashcard module exactly as the app
// wires it: mounted as the active module on the real control server, reached
// through the /game/ and /api/game/ prefixes (which the server strips).
func TestModuleUnderControlServer(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name, err := profile.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := control.NewServer(control.Deps{
		Name:  name,
		Video: control.NewVideoStore(""),
		Nav:   control.NewNavStore(nil),
	}, 0)
	srv.SetActiveModule(newModule(st))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// The SPA is served at /game/.
	res, err := http.Get(ts.URL + "/game/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "Flashcards") {
		t.Fatalf("/game/ status=%d, body has Flashcards=%v", res.StatusCode, strings.Contains(string(body), "Flashcards"))
	}

	// Its REST is reachable under /api/game/ (note the app-side double /api: the
	// SPA prefixes BASE=/api/game onto its existing /api/cards path).
	res, err = http.Get(ts.URL + "/api/game/api/cards")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("/api/game/api/cards status=%d", res.StatusCode)
	}
}
