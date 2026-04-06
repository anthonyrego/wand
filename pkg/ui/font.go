package ui

import (
	"strings"

	"github.com/anthonyrego/wand/pkg/mesh"
	"github.com/anthonyrego/wand/pkg/renderer"
	"github.com/anthonyrego/wand/pkg/sign"
)

const fontSpacing = 1 // pixels between characters

// NewTextMesh creates a 2D text mesh in screen-pixel space.
// Each font pixel becomes a pixelSize x pixelSize screen quad.
// Origin is top-left of the text. Returns the mesh and total width in screen pixels.
func NewTextMesh(r *renderer.Renderer, text string, pixelSize float32, cr, cg, cb, ca uint8) (*mesh.Mesh, float32, error) {
	text = strings.ToUpper(text)
	if len(text) == 0 {
		text = " "
	}

	var vertices []renderer.Vertex
	var indices []uint16

	cursorX := float32(0)

	for i := 0; i < len(text); i++ {
		ch := text[i]
		glyph, ok := sign.Glyphs[ch]
		if !ok {
			cursorX += float32(sign.CharWidth+fontSpacing) * pixelSize
			continue
		}

		for row := 0; row < sign.CharHeight; row++ {
			bits := glyph[row]
			y := float32(row) * pixelSize

			for col := 0; col < sign.CharWidth; col++ {
				if bits&(1<<(sign.CharWidth-1-col)) != 0 {
					x := cursorX + float32(col)*pixelSize

					base := uint16(len(vertices))
					vertices = append(vertices,
						renderer.Vertex{X: x, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
						renderer.Vertex{X: x + pixelSize, Y: y, Z: 0, R: cr, G: cg, B: cb, A: ca},
						renderer.Vertex{X: x + pixelSize, Y: y + pixelSize, Z: 0, R: cr, G: cg, B: cb, A: ca},
						renderer.Vertex{X: x, Y: y + pixelSize, Z: 0, R: cr, G: cg, B: cb, A: ca},
					)
					indices = append(indices,
						base, base+1, base+2,
						base, base+2, base+3,
					)
				}
			}
		}
		cursorX += float32(sign.CharWidth+fontSpacing) * pixelSize
	}

	totalWidth := cursorX - float32(fontSpacing)*pixelSize

	if len(vertices) == 0 {
		vertices = append(vertices,
			renderer.Vertex{},
			renderer.Vertex{X: 1},
			renderer.Vertex{X: 1, Y: 1},
		)
		indices = append(indices, 0, 1, 2)
		totalWidth = float32(sign.CharWidth) * pixelSize
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
