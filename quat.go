package wand

import "math"

// quat represents a unit quaternion for 3D rotation.
type quat struct {
	W, X, Y, Z float32
}

func quatIdentity() quat {
	return quat{W: 1}
}

// eulerToQuat converts Euler angles (degrees) to a quaternion.
// Uses YXZ intrinsic rotation order matching the BNO085 convention.
func eulerToQuat(roll, pitch, yaw float32) quat {
	r := float64(roll) * math.Pi / 180 / 2
	p := float64(pitch) * math.Pi / 180 / 2
	y := float64(yaw) * math.Pi / 180 / 2

	cr, sr := math.Cos(r), math.Sin(r)
	cp, sp := math.Cos(p), math.Sin(p)
	cy, sy := math.Cos(y), math.Sin(y)

	return quat{
		W: float32(cy*cp*cr + sy*sp*sr),
		X: float32(cy*sp*cr + sy*cp*sr),
		Y: float32(sy*cp*cr - cy*sp*sr),
		Z: float32(cy*cp*sr - sy*sp*cr),
	}
}

// toEuler extracts Euler angles (degrees) in YXZ intrinsic order.
func (q quat) toEuler() (roll, pitch, yaw float32) {
	// Pitch from asin, clamped to avoid NaN at gimbal lock.
	sinP := 2 * (float64(q.W)*float64(q.X) - float64(q.Y)*float64(q.Z))
	if sinP > 1 {
		sinP = 1
	} else if sinP < -1 {
		sinP = -1
	}
	pitch = float32(math.Asin(sinP) * 180 / math.Pi)

	// Roll
	roll = float32(math.Atan2(
		2*(float64(q.X)*float64(q.Y)+float64(q.W)*float64(q.Z)),
		1-2*(float64(q.X)*float64(q.X)+float64(q.Z)*float64(q.Z)),
	) * 180 / math.Pi)

	// Yaw
	yaw = float32(math.Atan2(
		2*(float64(q.X)*float64(q.Z)+float64(q.W)*float64(q.Y)),
		1-2*(float64(q.X)*float64(q.X)+float64(q.Y)*float64(q.Y)),
	) * 180 / math.Pi)

	return roll, pitch, yaw
}

func (q quat) dot(other quat) float32 {
	return q.W*other.W + q.X*other.X + q.Y*other.Y + q.Z*other.Z
}

func (q quat) normalize() quat {
	lenSq := float64(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
	if lenSq < 1e-10 {
		return quatIdentity()
	}
	inv := float32(1 / math.Sqrt(lenSq))
	return quat{q.W * inv, q.X * inv, q.Y * inv, q.Z * inv}
}

// slerp performs spherical linear interpolation from q toward other.
// t=0 returns q, t=1 returns other. Always takes the shortest path.
func (q quat) slerp(other quat, t float32) quat {
	d := q.dot(other)

	// Ensure shortest path.
	if d < 0 {
		other = quat{-other.W, -other.X, -other.Y, -other.Z}
		d = -d
	}

	// Near-identical quaternions: fall back to normalized lerp.
	if d > 0.9995 {
		return quat{
			W: q.W + t*(other.W-q.W),
			X: q.X + t*(other.X-q.X),
			Y: q.Y + t*(other.Y-q.Y),
			Z: q.Z + t*(other.Z-q.Z),
		}.normalize()
	}

	theta := float32(math.Acos(float64(d)))
	sinTheta := float32(math.Sin(float64(theta)))
	w1 := float32(math.Sin(float64((1-t)*theta))) / sinTheta
	w2 := float32(math.Sin(float64(t*theta))) / sinTheta

	return quat{
		W: w1*q.W + w2*other.W,
		X: w1*q.X + w2*other.X,
		Y: w1*q.Y + w2*other.Y,
		Z: w1*q.Z + w2*other.Z,
	}
}
