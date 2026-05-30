package flashcard

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// exifAPP1 builds a minimal little-endian Exif APP1 segment carrying just the
// Orientation tag.
func exifAPP1(orientation byte) []byte {
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // TIFF header, IFD0 at offset 8
		0x01, 0x00, // 1 entry
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, orientation, 0x00, 0x00, 0x00, // Orientation (SHORT)
		0x00, 0x00, 0x00, 0x00, // next IFD = none
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segLen := len(payload) + 2
	return append([]byte{0xFF, 0xE1, byte(segLen >> 8), byte(segLen)}, payload...)
}

// makeJPEGWithOrientation encodes a JPEG and splices an Exif APP1 orientation
// segment in right after the SOI marker, mimicking a phone photo.
func makeJPEGWithOrientation(t *testing.T, w, h int, orientation byte) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	data := buf.Bytes()

	out := make([]byte, 0, len(data)+40)
	out = append(out, data[:2]...) // SOI
	out = append(out, exifAPP1(orientation)...)
	out = append(out, data[2:]...)
	return out
}

func TestExifOrientationReadsTag(t *testing.T) {
	for _, o := range []byte{1, 3, 6, 8} {
		if got := exifOrientation(makeJPEGWithOrientation(t, 8, 8, o)); got != int(o) {
			t.Fatalf("orientation %d: got %d", o, got)
		}
	}
}

func TestExifOrientationAbsent(t *testing.T) {
	// PNG has no EXIF.
	if got := exifOrientation(makePNG(t, 8, 8)); got != 1 {
		t.Fatalf("png orientation got %d, want 1", got)
	}
	// Plain JPEG with no spliced APP1.
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil)
	if got := exifOrientation(buf.Bytes()); got != 1 {
		t.Fatalf("plain jpeg orientation got %d, want 1", got)
	}
}

func TestApplyOrientationRotate90CW(t *testing.T) {
	// Horizontal [A B] should become vertical [A over B] after a 90° CW rotate.
	a := color.RGBA{255, 0, 0, 255}
	b := color.RGBA{0, 255, 0, 255}
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, a)
	src.Set(1, 0, b)

	dst := applyOrientation(src, 6)
	if dst.Rect.Dx() != 1 || dst.Rect.Dy() != 2 {
		t.Fatalf("dims after rotate = %dx%d, want 1x2", dst.Rect.Dx(), dst.Rect.Dy())
	}
	if dst.RGBAAt(0, 0) != a {
		t.Fatalf("top pixel = %v, want %v", dst.RGBAAt(0, 0), a)
	}
	if dst.RGBAAt(0, 1) != b {
		t.Fatalf("bottom pixel = %v, want %v", dst.RGBAAt(0, 1), b)
	}
}

func TestProcessUploadAppliesOrientation(t *testing.T) {
	// A 40x20 photo tagged orientation 6 must come out 20x40 (upright).
	d, _, err := processUpload(bytes.NewReader(makeJPEGWithOrientation(t, 40, 20, 6)))
	if err != nil {
		t.Fatalf("processUpload: %v", err)
	}
	if d.W != 20 || d.H != 40 {
		t.Fatalf("oriented size = %dx%d, want 20x40", d.W, d.H)
	}
	if len(d.Pix) != int(d.W*d.H*4) {
		t.Fatalf("Pix length = %d, want %d", len(d.Pix), d.W*d.H*4)
	}
}
