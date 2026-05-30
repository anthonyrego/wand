package flashcard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestProcessUploadDownscales(t *testing.T) {
	d, encoded, err := processUpload(bytes.NewReader(makePNG(t, 2000, 1000)))
	if err != nil {
		t.Fatalf("processUpload: %v", err)
	}
	if d.W != 1024 || d.H != 512 {
		t.Fatalf("expected 1024x512, got %dx%d", d.W, d.H)
	}
	if got, want := len(d.Pix), int(d.W*d.H*4); got != want {
		t.Fatalf("Pix length = %d, want %d (tight RGBA)", got, want)
	}
	// Re-encoded bytes must decode back to the downscaled size.
	cfg, err := png.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode re-encoded config: %v", err)
	}
	if cfg.Width != 1024 || cfg.Height != 512 {
		t.Fatalf("re-encoded size = %dx%d, want 1024x512", cfg.Width, cfg.Height)
	}
}

func TestProcessUploadSmallUnchanged(t *testing.T) {
	d, _, err := processUpload(bytes.NewReader(makePNG(t, 10, 20)))
	if err != nil {
		t.Fatalf("processUpload: %v", err)
	}
	if d.W != 10 || d.H != 20 {
		t.Fatalf("expected 10x20 unchanged, got %dx%d", d.W, d.H)
	}
	if len(d.Pix) != 10*20*4 {
		t.Fatalf("Pix length = %d, want %d", len(d.Pix), 10*20*4)
	}
}

func TestProcessUploadRejectsGarbage(t *testing.T) {
	if _, _, err := processUpload(strings.NewReader("not an image")); err == nil {
		t.Fatal("expected error decoding garbage, got nil")
	}
}
