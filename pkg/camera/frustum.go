package camera

import "github.com/go-gl/mathgl/mgl32"

// Plane represents a plane equation ax + by + cz + d = 0 (normalized).
type Plane struct {
	N mgl32.Vec3 // Normal (a, b, c)
	D float32    // Distance
}

// Frustum holds the six frustum planes extracted from a VP matrix.
type Frustum struct {
	Planes [6]Plane // left, right, bottom, top, near, far
}

// ExtractFrustum extracts frustum planes from a view-projection matrix.
// Planes point inward: a point is inside the frustum if dot(N, P) + D >= 0
// for all six planes.
func ExtractFrustum(vp mgl32.Mat4) Frustum {
	var f Frustum

	// Row access: vp is column-major, so vp[col*4+row]
	// row0 = (vp[0], vp[4], vp[8],  vp[12])
	// row1 = (vp[1], vp[5], vp[9],  vp[13])
	// row2 = (vp[2], vp[6], vp[10], vp[14])
	// row3 = (vp[3], vp[7], vp[11], vp[15])

	// Left:   row3 + row0
	f.Planes[0] = normalizePlane(
		vp[3]+vp[0], vp[7]+vp[4], vp[11]+vp[8], vp[15]+vp[12],
	)
	// Right:  row3 - row0
	f.Planes[1] = normalizePlane(
		vp[3]-vp[0], vp[7]-vp[4], vp[11]-vp[8], vp[15]-vp[12],
	)
	// Bottom: row3 + row1
	f.Planes[2] = normalizePlane(
		vp[3]+vp[1], vp[7]+vp[5], vp[11]+vp[9], vp[15]+vp[13],
	)
	// Top:    row3 - row1
	f.Planes[3] = normalizePlane(
		vp[3]-vp[1], vp[7]-vp[5], vp[11]-vp[9], vp[15]-vp[13],
	)
	// Near:   row2 (for [0,1] depth range)
	f.Planes[4] = normalizePlane(
		vp[2], vp[6], vp[10], vp[14],
	)
	// Far:    row3 - row2
	f.Planes[5] = normalizePlane(
		vp[3]-vp[2], vp[7]-vp[6], vp[11]-vp[10], vp[15]-vp[14],
	)

	return f
}

func normalizePlane(a, b, c, d float32) Plane {
	n := mgl32.Vec3{a, b, c}
	l := n.Len()
	if l < 1e-8 {
		return Plane{N: n, D: d}
	}
	return Plane{
		N: n.Mul(1 / l),
		D: d / l,
	}
}

// SphereVisible returns true if a bounding sphere is at least partially
// inside the frustum. Center is the sphere center, radius is its radius.
func (f *Frustum) SphereVisible(center mgl32.Vec3, radius float32) bool {
	for i := range f.Planes {
		dist := f.Planes[i].N.Dot(center) + f.Planes[i].D
		if dist < -radius {
			return false
		}
	}
	return true
}
