package profile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanName(t *testing.T) {
	cases := map[string]string{
		"":                      "",
		"  Emma  ":              "Emma",
		"Emma\n":                "Emma",
		"José":                  "Jos", // non-ASCII é stripped
		"a\tb":                  "ab",  // control char (tab) stripped
		strings.Repeat("x", 40): strings.Repeat("x", maxNameLen),
	}
	for in, want := range cases {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTitle(t *testing.T) {
	cases := map[string]string{
		"":      "WAND",
		"   ":   "WAND",
		"Emma":  "EMMA'S WAND",
		"lucas": "LUCAS'S WAND",
	}
	for in, want := range cases {
		if got := Title(in); got != want {
			t.Errorf("Title(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowTitle(t *testing.T) {
	if got := WindowTitle(""); got != "Wand" {
		t.Errorf("WindowTitle(\"\") = %q, want %q", got, "Wand")
	}
	if got := WindowTitle("Emma"); got != "Emma's Wand" {
		t.Errorf("WindowTitle(\"Emma\") = %q, want %q", got, "Emma's Wand")
	}
}

func TestStorePersistsAcrossLoad(t *testing.T) {
	dir := t.TempDir()

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name() != "" || s.Title() != "WAND" {
		t.Fatalf("fresh store: name=%q title=%q", s.Name(), s.Title())
	}

	rev := s.Revision()
	got := s.SetName("  Emma  ")
	if got != "Emma" {
		t.Fatalf("SetName returned %q, want %q", got, "Emma")
	}
	if s.Revision() == rev {
		t.Fatalf("Revision did not change after SetName")
	}

	// Setting the same (cleaned) name again should not bump the revision.
	rev2 := s.Revision()
	s.SetName("Emma")
	if s.Revision() != rev2 {
		t.Fatalf("Revision changed on no-op SetName")
	}

	// A fresh Load from the same dir must see the persisted name.
	s2, err := Load(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if s2.Name() != "Emma" || s2.Title() != "EMMA'S WAND" {
		t.Fatalf("reloaded store: name=%q title=%q", s2.Name(), s2.Title())
	}

	// Sanity check the on-disk shape.
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var p persisted
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.OwnerName != "Emma" {
		t.Fatalf("on-disk ownerName=%q, want %q", p.OwnerName, "Emma")
	}
}

func TestServerGetSet(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	srv := httptest.NewServer(NewServer(s, 0).Handler())
	defer srv.Close()

	// POST a name.
	resp, err := http.Post(srv.URL+"/api/profile", "application/json",
		strings.NewReader(`{"name":"Emma"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var set profileDTO
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		t.Fatalf("decode POST: %v", err)
	}
	resp.Body.Close()
	if set.Name != "Emma" || set.Title != "EMMA'S WAND" {
		t.Fatalf("POST result: %+v", set)
	}
	if s.Name() != "Emma" {
		t.Fatalf("store not updated: %q", s.Name())
	}

	// GET reflects it.
	resp, err = http.Get(srv.URL + "/api/profile")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	var get profileDTO
	if err := json.NewDecoder(resp.Body).Decode(&get); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	resp.Body.Close()
	if get.Name != "Emma" || get.Title != "EMMA'S WAND" {
		t.Fatalf("GET result: %+v", get)
	}

	// Index page is served.
	resp, err = http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d", resp.StatusCode)
	}
}
