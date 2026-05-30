package ui

import (
	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/sign"
)

const (
	// FontRows is the glyph height (in font pixels) of the shared font.
	FontRows = sign.CharHeight

	// FontScale is the default on-screen pixel size for one font pixel, used by
	// the title/labels on the splash and game overlays.
	FontScale = 3

	fontGap      = 1 // blank columns between glyphs
	fontSpaceCol = 4 // advance for space / unknown glyphs
)

// TextWidth returns the rendered width in screen pixels of text at the given
// pixel size, without building a mesh (mirrors NewTextMesh's advance logic).
func TextWidth(text string, pixelSize float32) float32 {
	cursorX := float32(0)
	for _, ch := range text {
		g, ok := sign.Glyphs[byte(ch)]
		if ch < 0 || ch > 0x7E || !ok {
			cursorX += float32(fontSpaceCol+fontGap) * pixelSize
			continue
		}
		w := sign.GlyphWidth(g)
		if w == 0 {
			cursorX += float32(fontSpaceCol+fontGap) * pixelSize
			continue
		}
		cursorX += float32(w+fontGap) * pixelSize
	}
	width := cursorX - float32(fontGap)*pixelSize
	if width < 0 {
		return 0
	}
	return width
}

// NewTextMesh creates a 2D text mesh in screen-pixel space using the shared 8x8
// font with proportional spacing. Each font pixel becomes a pixelSize x
// pixelSize screen quad. Origin is the top-left of the text. Case is preserved
// (the font includes lowercase). Returns the mesh and total width in screen
// pixels.
func NewTextMesh(r *renderer.Renderer, text string, pixelSize float32, cr, cg, cb, ca uint8) (*mesh.Mesh, float32, error) {
	var vertices []renderer.Vertex
	var indices []uint16

	cursorX := float32(0)
	for _, ch := range text {
		g, ok := sign.Glyphs[byte(ch)]
		if ch < 0 || ch > 0x7E || !ok {
			cursorX += float32(fontSpaceCol+fontGap) * pixelSize
			continue
		}
		w := sign.GlyphWidth(g)
		if w == 0 { // space
			cursorX += float32(fontSpaceCol+fontGap) * pixelSize
			continue
		}

		for row := 0; row < sign.CharHeight; row++ {
			bits := g[row]
			y := float32(row) * pixelSize
			for col := 0; col < sign.CharWidth; col++ {
				if bits&(1<<col) == 0 {
					continue
				}
				x := cursorX + float32(col)*pixelSize
				base := uint16(len(vertices))
				vertices = append(vertices,
					renderer.Vertex{X: x, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
					renderer.Vertex{X: x + pixelSize, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
					renderer.Vertex{X: x + pixelSize, Y: y + pixelSize, Z: 0, R: cr, G: cg, B: cb, A: ca},
					renderer.Vertex{X: x, Y: y + pixelSize, Z: 0, R: cr, G: cg, B: cb, A: ca},
				)
				indices = append(indices, base, base+1, base+2, base, base+2, base+3)
			}
		}
		cursorX += float32(w+fontGap) * pixelSize
	}

	totalWidth := cursorX - float32(fontGap)*pixelSize
	if totalWidth < 0 {
		totalWidth = 0
	}

	if len(vertices) == 0 {
		// Degenerate triangle keeps the GPU buffers non-empty.
		vertices = append(vertices,
			renderer.Vertex{},
			renderer.Vertex{X: 1},
			renderer.Vertex{X: 1, Y: 1},
		)
		indices = append(indices, 0, 1, 2)
		totalWidth = float32(fontSpaceCol) * pixelSize
	}

	vertexBuffer, err := r.CreateVertexBuffer(vertices)
	if err != nil {
		return nil, 0, err
	}

	indexBuffer, err := r.CreateIndexBuffer(indices)
	if err != nil {
		r.ReleaseBuffer(vertexBuffer)
		return nil, 0, err
	}

	return &mesh.Mesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   uint32(len(indices)),
	}, totalWidth, nil
}
