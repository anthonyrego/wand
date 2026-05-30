package control

import (
	"image"
	"image/draw"

	qrcode "github.com/skip2/go-qrcode"
)

// QRCodeRGBA encodes text as a QR code and returns it as a contiguous RGBA
// pixel buffer (black modules on white) roughly size x size, ready for upload
// via renderer.CreateTextureFromRGBA. It touches no GPU/SDL state.
//
// The returned dimensions may differ slightly from size: go-qrcode rounds to a
// whole number of modules. Callers should use the returned w/h, not size.
func QRCodeRGBA(text string, size int) (w, h uint32, pix []byte, err error) {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return 0, 0, nil, err
	}
	src := q.Image(size)
	b := src.Bounds()

	// Draw onto a fresh RGBA so the buffer is guaranteed contiguous (stride ==
	// 4*width, no padding), regardless of the concrete type Image() returns.
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	return uint32(b.Dx()), uint32(b.Dy()), rgba.Pix, nil
}
