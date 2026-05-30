package profile

import (
	"encoding/json"
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
		"":      "TOY BOX",
		"   ":   "TOY BOX",
		"Emma":  "EMMA'S TOY BOX",
		"lucas": "LUCAS'S TOY BOX",
	}
	for in, want := range cases {
		if got := Title(in); got != want {
			t.Errorf("Title(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowTitle(t *testing.T) {
	if got := WindowTitle(""); got != "Toy Box" {
		t.Errorf("WindowTitle(\"\") = %q, want %q", got, "Toy Box")
	}
	if got := WindowTitle("Emma"); got != "Emma's Toy Box" {
		t.Errorf("WindowTitle(\"Emma\") = %q, want %q", got, "Emma's Toy Box")
	}
}

func TestStorePersistsAcrossLoad(t *testing.T) {
	dir := t.TempDir()

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name() != "" || s.Title() != "TOY BOX" {
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
	if s2.Name() != "Emma" || s2.Title() != "EMMA'S TOY BOX" {
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
