package mesh

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/anthonyrego/wand/pkg/renderer"
)

type Mesh struct {
	VertexBuffer *sdl.GPUBuffer
	IndexBuffer  *sdl.GPUBuffer
	IndexCount   uint32
}

func NewCube(r *renderer.Renderer, red, green, blue uint8) (*Mesh, error) {
	// Cube vertices with position and color
	// Each face has a slightly different shade for visibility
	vertices := []renderer.Vertex{
		// Front face (Z+)
		renderer.NewVertex(-0.5, -0.5, 0.5, red, green, blue, 255),
		renderer.NewVertex(0.5, -0.5, 0.5, red, green, blue, 255),
		renderer.NewVertex(0.5, 0.5, 0.5, red, green, blue, 255),
		renderer.NewVertex(-0.5, 0.5, 0.5, red, green, blue, 255),

		// Back face (Z-)
		renderer.NewVertex(0.5, -0.5, -0.5, uint8(float32(red)*0.8), uint8(float32(green)*0.8), uint8(float32(blue)*0.8), 255),
		renderer.NewVertex(-0.5, -0.5, -0.5, uint8(float32(red)*0.8), uint8(float32(green)*0.8), uint8(float32(blue)*0.8), 255),
		renderer.NewVertex(-0.5, 0.5, -0.5, uint8(float32(red)*0.8), uint8(float32(green)*0.8), uint8(float32(blue)*0.8), 255),
		renderer.NewVertex(0.5, 0.5, -0.5, uint8(float32(red)*0.8), uint8(float32(green)*0.8), uint8(float32(blue)*0.8), 255),

		// Top face (Y+)
		renderer.NewVertex(-0.5, 0.5, 0.5, uint8(float32(red)*0.9), uint8(float32(green)*0.9), uint8(float32(blue)*0.9), 255),
		renderer.NewVertex(0.5, 0.5, 0.5, uint8(float32(red)*0.9), uint8(float32(green)*0.9), uint8(float32(blue)*0.9), 255),
		renderer.NewVertex(0.5, 0.5, -0.5, uint8(float32(red)*0.9), uint8(float32(green)*0.9), uint8(float32(blue)*0.9), 255),
		renderer.NewVertex(-0.5, 0.5, -0.5, uint8(float32(red)*0.9), uint8(float32(green)*0.9), uint8(float32(blue)*0.9), 255),

		// Bottom face (Y-)
		renderer.NewVertex(-0.5, -0.5, -0.5, uint8(float32(red)*0.6), uint8(float32(green)*0.6), uint8(float32(blue)*0.6), 255),
		renderer.NewVertex(0.5, -0.5, -0.5, uint8(float32(red)*0.6), uint8(float32(green)*0.6), uint8(float32(blue)*0.6), 255),
		renderer.NewVertex(0.5, -0.5, 0.5, uint8(float32(red)*0.6), uint8(float32(green)*0.6), uint8(float32(blue)*0.6), 255),
		renderer.NewVertex(-0.5, -0.5, 0.5, uint8(float32(red)*0.6), uint8(float32(green)*0.6), uint8(float32(blue)*0.6), 255),

		// Right face (X+)
		renderer.NewVertex(0.5, -0.5, 0.5, uint8(float32(red)*0.85), uint8(float32(green)*0.85), uint8(float32(blue)*0.85), 255),
		renderer.NewVertex(0.5, -0.5, -0.5, uint8(float32(red)*0.85), uint8(float32(green)*0.85), uint8(float32(blue)*0.85), 255),
		renderer.NewVertex(0.5, 0.5, -0.5, uint8(float32(red)*0.85), uint8(float32(green)*0.85), uint8(float32(blue)*0.85), 255),
		renderer.NewVertex(0.5, 0.5, 0.5, uint8(float32(red)*0.85), uint8(float32(green)*0.85), uint8(float32(blue)*0.85), 255),

		// Left face (X-)
		renderer.NewVertex(-0.5, -0.5, -0.5, uint8(float32(red)*0.7), uint8(float32(green)*0.7), uint8(float32(blue)*0.7), 255),
		renderer.NewVertex(-0.5, -0.5, 0.5, uint8(float32(red)*0.7), uint8(float32(green)*0.7), uint8(float32(blue)*0.7), 255),
		renderer.NewVertex(-0.5, 0.5, 0.5, uint8(float32(red)*0.7), uint8(float32(green)*0.7), uint8(float32(blue)*0.7), 255),
		renderer.NewVertex(-0.5, 0.5, -0.5, uint8(float32(red)*0.7), uint8(float32(green)*0.7), uint8(float32(blue)*0.7), 255),
	}

	indices := []uint16{
		// Front
		0, 1, 2, 0, 2, 3,
		// Back
		4, 5, 6, 4, 6, 7,
		// Top
		8, 9, 10, 8, 10, 11,
		// Bottom
		12, 13, 14, 12, 14, 15,
		// Right
		16, 17, 18, 16, 18, 19,
		// Left
		20, 21, 22, 20, 22, 23,
	}

	vertexBuffer, err := r.CreateVertexBuffer(vertices)
	if err != nil {
		return nil, err
	}

	indexBuffer, err := r.CreateIndexBuffer(indices)
	if err != nil {
		r.ReleaseBuffer(vertexBuffer)
		return nil, err
	}

	return &Mesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   uint32(len(indices)),
	}, nil
}

func NewLitCube(r *renderer.Renderer, red, green, blue uint8) (*Mesh, error) {
	vertices := []renderer.LitVertex{
		// Front face (Z+) — normal (0, 0, 1)
		{X: -0.5, Y: -0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 0, NZ: 1, R: red, G: green, B: blue, A: 255},

		// Back face (Z-) — normal (0, 0, -1)
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: -0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 0, NZ: -1, R: red, G: green, B: blue, A: 255},

		// Top face (Y+) — normal (0, 1, 0)
		{X: -0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},

		// Bottom face (Y-) — normal (0, -1, 0)
		{X: -0.5, Y: -0.5, Z: -0.5, NX: 0, NY: -1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 0, NY: -1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 0, NY: -1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: -0.5, Z: 0.5, NX: 0, NY: -1, NZ: 0, R: red, G: green, B: blue, A: 255},

		// Right face (X+) — normal (1, 0, 0)
		{X: 0.5, Y: -0.5, Z: 0.5, NX: 1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: -0.5, Z: -0.5, NX: 1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: -0.5, NX: 1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: 0.5, Y: 0.5, Z: 0.5, NX: 1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},

		// Left face (X-) — normal (-1, 0, 0)
		{X: -0.5, Y: -0.5, Z: -0.5, NX: -1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: -0.5, Z: 0.5, NX: -1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: 0.5, Z: 0.5, NX: -1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -0.5, Y: 0.5, Z: -0.5, NX: -1, NY: 0, NZ: 0, R: red, G: green, B: blue, A: 255},
	}

	indices := []uint16{
		0, 1, 2, 0, 2, 3, // Front
		4, 5, 6, 4, 6, 7, // Back
		8, 9, 10, 8, 10, 11, // Top
		12, 13, 14, 12, 14, 15, // Bottom
		16, 17, 18, 16, 18, 19, // Right
		20, 21, 22, 20, 22, 23, // Left
	}

	vertexBuffer, err := r.CreateLitVertexBuffer(vertices)
	if err != nil {
		return nil, err
	}

	indexBuffer, err := r.CreateIndexBuffer(indices)
	if err != nil {
		r.ReleaseBuffer(vertexBuffer)
		return nil, err
	}

	return &Mesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   uint32(len(indices)),
	}, nil
}

func NewGroundPlane(r *renderer.Renderer, size float32, red, green, blue uint8) (*Mesh, error) {
	vertices := []renderer.LitVertex{
		{X: -size, Y: 0, Z: size, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: size, Y: 0, Z: size, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: size, Y: 0, Z: -size, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
		{X: -size, Y: 0, Z: -size, NX: 0, NY: 1, NZ: 0, R: red, G: green, B: blue, A: 255},
	}

	indices := []uint16{0, 1, 2, 0, 2, 3}

	vertexBuffer, err := r.CreateLitVertexBuffer(vertices)
	if err != nil {
		return nil, err
	}

	indexBuffer, err := r.CreateIndexBuffer(indices)
	if err != nil {
		r.ReleaseBuffer(vertexBuffer)
		return nil, err
	}

	return &Mesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   uint32(len(indices)),
	}, nil
}

// NewSkyDome creates an inverted sphere for use as a sky backdrop.
// Vertex colors interpolate from horizon (bottom) to zenith (top).
// All normals point (0,-1,0) so only ambient light contributes — no sun/point-light artifacts.
func NewSkyDome(r *renderer.Renderer, radius float32, horizonR, horizonG, horizonB, zenithR, zenithG, zenithB uint8) (*Mesh, error) {
	const rings = 16
	const segments = 24

	var vertices []renderer.LitVertex
	var indices []uint16

	// Generate vertices: rings from bottom (-Y) to top (+Y)
	for ring := 0; ring <= rings; ring++ {
		// phi goes from PI (bottom) to 0 (top)
		phi := math.Pi * (1.0 - float64(ring)/float64(rings))
		y := float64(radius) * math.Cos(phi)
		ringRadius := float64(radius) * math.Sin(phi)

		// t=0 at bottom (horizon), t=1 at top (zenith)
		t := float64(ring) / float64(rings)
		// Use smoothstep for a nicer gradient curve
		t = t * t * (3 - 2*t)
		cr := uint8(float64(horizonR)*(1-t) + float64(zenithR)*t)
		cg := uint8(float64(horizonG)*(1-t) + float64(zenithG)*t)
		cb := uint8(float64(horizonB)*(1-t) + float64(zenithB)*t)

		for seg := 0; seg <= segments; seg++ {
			theta := 2 * math.Pi * float64(seg) / float64(segments)
			x := ringRadius * math.Sin(theta)
			z := ringRadius * math.Cos(theta)

			vertices = append(vertices, renderer.LitVertex{
				X: float32(x), Y: float32(y), Z: float32(z),
				NX: 0, NY: -1, NZ: 0, // all normals down — suppresses sun/point light
				R: cr, G: cg, B: cb, A: 255,
			})
		}
	}

	// Generate indices (inverted winding for inside-facing triangles)
	for ring := 0; ring < rings; ring++ {
		for seg := 0; seg < segments; seg++ {
			curr := uint16(ring*(segments+1) + seg)
			next := curr + uint16(segments+1)

			// Inward-facing winding (front faces visible from inside)
			indices = append(indices, curr, next, curr+1)
			indices = append(indices, curr+1, next, next+1)
		}
	}

	vertexBuffer, err := r.CreateLitVertexBuffer(vertices)
	if err != nil {
		return nil, err
	}

	indexBuffer, err := r.CreateIndexBuffer(indices)
	if err != nil {
		r.ReleaseBuffer(vertexBuffer)
		return nil, err
	}

	return &Mesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   uint32(len(indices)),
	}, nil
}

func (m *Mesh) Destroy(r *renderer.Renderer) {
	r.ReleaseBuffer(m.VertexBuffer)
	r.ReleaseBuffer(m.IndexBuffer)
}
