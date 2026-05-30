package flashcard

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/gif"  // register GIF decoder
	_ "image/jpeg" // register JPEG decoder
	"image/png"
	"io"
	"math"
)

// maxDim is the largest dimension (px) a stored card image may have. Uploads
// larger than this are downscaled. Keeps GPU textures and memory reasonable
// while staying well above the on-screen display resolution.
const maxDim = 1024

// decoded is a card image in canonical, tightly-packed RGBA form, ready to hand
// to the GPU via Renderer.CreateTextureFromRGBA. Pix has no row padding
// (Stride == 4*W) and the rectangle origin is (0,0), which the GPU upload path
// assumes (PixelsPerRow = width).
type decoded struct {
	W, H uint32
	Pix  []byte
}

// processUpload decodes an uploaded image (PNG/JPEG/GIF), applies any EXIF
// orientation (phone photos are often stored sideways with an orientation tag),
// downscales it to fit within maxDim, and returns both the GPU-ready RGBA and a
// PNG re-encoding for on-disk storage (uniform, orientation-free format).
func processUpload(r io.Reader) (*decoded, []byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	rgba := toRGBA(img)
	if o := exifOrientation(data); o > 1 {
		rgba = applyOrientation(rgba, o)
	}
	rgba = resizeMax(rgba, maxDim)

	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, nil, err
	}

	return rgbaToDecoded(rgba), buf.Bytes(), nil
}

// decodeStored reads an already-processed image file from disk into RGBA. No
// resizing — files on disk were downscaled at upload time.
func decodeStored(r io.Reader) (*decoded, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	return rgbaToDecoded(toRGBA(img)), nil
}

func rgbaToDecoded(img *image.RGBA) *decoded {
	return &decoded{
		W:   uint32(img.Rect.Dx()),
		H:   uint32(img.Rect.Dy()),
		Pix: img.Pix,
	}
}

// toRGBA returns img as an *image.RGBA with origin (0,0) and a tight stride,
// copying if necessary. This guarantees Pix is a contiguous width*height*4
// buffer suitable for direct GPU upload.
func toRGBA(img image.Image) *image.RGBA {
	if src, ok := img.(*image.RGBA); ok &&
		src.Rect.Min == (image.Point{}) && src.Stride == 4*src.Rect.Dx() {
		return src
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// resizeMax downscales src so neither dimension exceeds max, preserving aspect
// ratio via area averaging (box filter). Images already within bounds are
// returned unchanged. Only ever downscales — never enlarges.
func resizeMax(src *image.RGBA, max int) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	if w <= max && h <= max {
		return src
	}

	scale := float64(max) / float64(maxInt(w, h))
	nw := maxInt(1, int(math.Round(float64(w)*scale)))
	nh := maxInt(1, int(math.Round(float64(h)*scale)))

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy0 := y * h / nh
		sy1 := (y + 1) * h / nh
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := 0; x < nw; x++ {
			sx0 := x * w / nw
			sx1 := (x + 1) * w / nw
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}

			var r, g, b, a, cnt uint32
			for sy := sy0; sy < sy1; sy++ {
				row := src.PixOffset(sx0, sy)
				for sx := sx0; sx < sx1; sx++ {
					r += uint32(src.Pix[row])
					g += uint32(src.Pix[row+1])
					b += uint32(src.Pix[row+2])
					a += uint32(src.Pix[row+3])
					cnt++
					row += 4
				}
			}

			di := dst.PixOffset(x, y)
			dst.Pix[di] = uint8(r / cnt)
			dst.Pix[di+1] = uint8(g / cnt)
			dst.Pix[di+2] = uint8(b / cnt)
			dst.Pix[di+3] = uint8(a / cnt)
		}
	}
	return dst
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
