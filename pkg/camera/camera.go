package camera

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

type Camera struct {
	Position mgl32.Vec3
	Yaw      float32 // Rotation around Y axis (left/right)
	Pitch    float32 // Rotation around X axis (up/down)

	MoveSpeed   float32
	LookSpeed   float32
	AspectRatio float32
	FOV         float32
	Near        float32
	Far         float32
}

func New(aspectRatio float32) *Camera {
	return &Camera{
		Position:    mgl32.Vec3{0, 0, 5},
		Yaw:         -90, // Looking towards -Z
		Pitch:       0,
		MoveSpeed:   20.0,
		LookSpeed:   0.1,
		AspectRatio: aspectRatio,
		FOV:         45.0,
		Near:        0.1,
		Far:         1000.0,
	}
}

func (c *Camera) Forward() mgl32.Vec3 {
	yawRad := float64(c.Yaw) * math.Pi / 180.0
	pitchRad := float64(c.Pitch) * math.Pi / 180.0

	return mgl32.Vec3{
		float32(math.Cos(yawRad) * math.Cos(pitchRad)),
		float32(math.Sin(pitchRad)),
		float32(math.Sin(yawRad) * math.Cos(pitchRad)),
	}.Normalize()
}

func (c *Camera) Right() mgl32.Vec3 {
	return c.Forward().Cross(mgl32.Vec3{0, 1, 0}).Normalize()
}

func (c *Camera) Up() mgl32.Vec3 {
	return c.Right().Cross(c.Forward()).Normalize()
}

func (c *Camera) Move(forward, right, up float32, deltaTime float32) {
	speed := c.MoveSpeed * deltaTime

	// Get movement directions
	fwd := c.Forward()
	rgt := c.Right()

	// Apply movement
	c.Position = c.Position.Add(fwd.Mul(forward * speed))
	c.Position = c.Position.Add(rgt.Mul(right * speed))
	c.Position = c.Position.Add(mgl32.Vec3{0, up * speed, 0})
}

func (c *Camera) Look(deltaX, deltaY float32) {
	c.Yaw += deltaX * c.LookSpeed
	c.Pitch -= deltaY * c.LookSpeed

	// Clamp pitch to prevent flipping
	if c.Pitch > 89.0 {
		c.Pitch = 89.0
	}
	if c.Pitch < -89.0 {
		c.Pitch = -89.0
	}
}

func (c *Camera) ViewMatrix() mgl32.Mat4 {
	target := c.Position.Add(c.Forward())
	return mgl32.LookAtV(c.Position, target, mgl32.Vec3{0, 1, 0})
}

func (c *Camera) ProjectionMatrix() mgl32.Mat4 {
	return ReversedZPerspective(
		mgl32.DegToRad(c.FOV),
		c.AspectRatio,
		c.Near,
		c.Far,
	)
}

// ReversedZPerspective creates a perspective projection matrix with reversed-Z
// for [0,1] depth range. Near maps to z=1, far maps to z=0, giving much better
// depth precision at large distances than standard projection.
func ReversedZPerspective(fovy, aspect, near, far float32) mgl32.Mat4 {
	f := float32(1.0 / math.Tan(float64(fovy/2)))
	return mgl32.Mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, near / (far - near), -1,
		0, 0, far * near / (far - near), 0,
	}
}

func (c *Camera) ViewProjectionMatrix() mgl32.Mat4 {
	return c.ProjectionMatrix().Mul4(c.ViewMatrix())
}
