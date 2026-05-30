package sign

import (
	"strings"
	"testing"
)

// renderASCII lays out a string using the shared font as block/space art, for
// visual verification of glyph shapes.
func renderASCII(s string) string {
	rows := make([]string, CharHeight)
	for _, ch := range s {
		g, ok := Glyphs[byte(ch)]
		if !ok {
			g = Glyphs[' ']
		}
		w := GlyphWidth(g)
		if w == 0 {
			w = 4
		}
		for r := 0; r < CharHeight; r++ {
			var b strings.Builder
			for c := 0; c < w; c++ {
				if g[r]&(1<<c) != 0 {
					b.WriteRune('#')
				} else {
					b.WriteRune(' ')
				}
			}
			b.WriteByte(' ')
			rows[r] += b.String()
		}
	}
	return strings.Join(rows, "\n")
}

func TestFontSample(t *testing.T) {
	for _, s := range []string{"Apple", "Grandma", "PAUSED", "http://192.168.1.5:8080"} {
		t.Logf("\n%q:\n%s\n", s, renderASCII(s))
	}
}

func TestFontCoversPrintableASCII(t *testing.T) {
	for c := byte(0x20); c <= 0x7E; c++ {
		if _, ok := Glyphs[c]; !ok {
			t.Errorf("missing glyph for %q (0x%02X)", string(rune(c)), c)
		}
	}
}

func TestGlyphWidth(t *testing.T) {
	if w := GlyphWidth(Glyphs[' ']); w != 0 {
		t.Errorf("space width = %d, want 0", w)
	}
	if w := GlyphWidth(Glyphs['W']); w < 5 {
		t.Errorf("'W' width = %d, want a wide glyph", w)
	}
}
