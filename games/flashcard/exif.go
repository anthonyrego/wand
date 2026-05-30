package flashcard

import (
	"encoding/binary"
	"image"
)

// exifOrientation extracts the EXIF Orientation tag (1–8) from JPEG bytes,
// returning 1 (normal) when absent or unparseable. Phone cameras commonly store
// photos in the sensor's native orientation and set this tag instead of
// rotating pixels; Go's JPEG decoder ignores it, so we read it ourselves and
// rotate during upload (see applyOrientation).
//
// This is a deliberately minimal reader: it walks JPEG markers to the Exif APP1
// segment and reads only tag 0x0112 from IFD0. It does not depend on a full
// EXIF library.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 { // SOI
		return 1
	}

	i := 2
	for i+4 <= len(data) {
		// Skip any 0xFF fill bytes before the marker code.
		for i+1 < len(data) && data[i] == 0xFF && data[i+1] == 0xFF {
			i++
		}
		if i+4 > len(data) || data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		// SOS (start of scan) or EOI — no more metadata worth scanning.
		if marker == 0xDA || marker == 0xD9 {
			return 1
		}
		// Standalone markers (RSTn, TEM) carry no length.
		if (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			i += 2
			continue
		}

		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen < 2 {
			return 1
		}
		segEnd := i + 2 + segLen
		if segEnd > len(data) {
			return 1
		}

		if marker == 0xE1 { // APP1
			seg := data[i+4 : segEnd]
			if len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
				if o, ok := parseTIFFOrientation(seg[6:]); ok {
					return o
				}
			}
		}
		i = segEnd
	}
	return 1
}

// parseTIFFOrientation reads the Orientation tag (0x0112) from a TIFF header
// (the body of an Exif APP1 segment, after the "Exif\0\0" prefix).
func parseTIFFOrientation(b []byte) (int, bool) {
	if len(b) < 8 {
		return 0, false
	}

	var bo binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0, false
	}
	if bo.Uint16(b[2:4]) != 0x002A {
		return 0, false
	}

	ifd := int(bo.Uint32(b[4:8]))
	if ifd < 8 || ifd+2 > len(b) {
		return 0, false
	}
	count := int(bo.Uint16(b[ifd : ifd+2]))
	entry := ifd + 2
	for k := 0; k < count; k++ {
		if entry+12 > len(b) {
			break
		}
		if bo.Uint16(b[entry:entry+2]) == 0x0112 { // Orientation, type SHORT
			val := int(bo.Uint16(b[entry+8 : entry+10]))
			if val >= 1 && val <= 8 {
				return val, true
			}
			return 0, false
		}
		entry += 12
	}
	return 0, false
}

// applyOrientation returns src transformed so the EXIF orientation is "baked
// in" — i.e. the result displays upright with no orientation metadata. For
// orientations 5–8 the width and height are swapped. Orientation 1 (or any
// out-of-range value) returns src unchanged.
func applyOrientation(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}

	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 { // 5,6,7,8 transpose the axes
		dw, dh = h, w
	}

	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			var sx, sy int
			switch o {
			case 2: // flip horizontal
				sx, sy = w-1-dx, dy
			case 3: // rotate 180
				sx, sy = w-1-dx, h-1-dy
			case 4: // flip vertical
				sx, sy = dx, h-1-dy
			case 5: // transpose
				sx, sy = dy, dx
			case 6: // rotate 90 CW
				sx, sy = dy, h-1-dx
			case 7: // transverse
				sx, sy = w-1-dy, h-1-dx
			case 8: // rotate 90 CCW
				sx, sy = w-1-dy, dx
			}
			si := src.PixOffset(sx, sy)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}
