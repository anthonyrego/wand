package camera

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// ScreenToWorldRay converts a screen pixel coordinate to a world-space ray.
// Returns the ray origin (near plane) and normalized direction.
func ScreenToWorldRay(screenX, screenY float32, screenW, screenH int, invVP mgl32.Mat4) (origin, dir mgl32.Vec3) {
	// Convert screen coords to NDC [-1, 1]
	ndcX := 2*screenX/float32(screenW) - 1
	ndcY := 1 - 2*screenY/float32(screenH)

	// Reversed-Z: near plane is z=1, far plane is z=0
	nearClip := mgl32.Vec4{ndcX, ndcY, 1, 1}
	farClip := mgl32.Vec4{ndcX, ndcY, 0, 1}

	// Unproject via inverse VP
	nearWorld := invVP.Mul4x1(nearClip)
	farWorld := invVP.Mul4x1(farClip)

	// Perspective divide
	if nearWorld[3] != 0 {
		nearWorld = nearWorld.Mul(1 / nearWorld[3])
	}
	if farWorld[3] != 0 {
		farWorld = farWorld.Mul(1 / farWorld[3])
	}

	origin = mgl32.Vec3{nearWorld[0], nearWorld[1], nearWorld[2]}
	farPt := mgl32.Vec3{farWorld[0], farWorld[1], farWorld[2]}
	dir = farPt.Sub(origin).Normalize()
	return
}

// RayTriangleIntersect tests a ray against a triangle using the Möller-Trumbore
// algorithm. Returns the distance t along the ray and whether it hit.
func RayTriangleIntersect(origin, dir, v0, v1, v2 mgl32.Vec3) (t float32, hit bool) {
	const epsilon = 1e-6

	edge1 := v1.Sub(v0)
	edge2 := v2.Sub(v0)
	h := cross(dir, edge2)
	a := dot(edge1, h)

	if a > -epsilon && a < epsilon {
		return 0, false // Ray parallel to triangle
	}

	f := 1.0 / a
	s := origin.Sub(v0)
	u := f * dot(s, h)

	if u < 0 || u > 1 {
		return 0, false
	}

	q := cross(s, edge1)
	v := f * dot(dir, q)

	if v < 0 || u+v > 1 {
		return 0, false
	}

	t = f * dot(edge2, q)
	if t > epsilon {
		return t, true
	}
	return 0, false
}

func cross(a, b mgl32.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func dot(a, b mgl32.Vec3) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// RaySphereIntersect tests a ray against a sphere. Returns the distance along
// the ray to the nearest intersection point and whether it hit.
func RaySphereIntersect(origin, dir, center mgl32.Vec3, radius float32) (t float32, hit bool) {
	oc := origin.Sub(center)
	b := 2 * oc.Dot(dir)
	c := oc.Dot(oc) - radius*radius
	discriminant := b*b - 4*c

	if discriminant < 0 {
		return 0, false
	}

	sqrtD := float32(math.Sqrt(float64(discriminant)))

	t1 := (-b - sqrtD) / 2
	t2 := (-b + sqrtD) / 2

	if t1 > 0 {
		return t1, true
	}
	if t2 > 0 {
		return t2, true
	}
	return 0, false
}
