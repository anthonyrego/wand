package wand

import (
	"math"
	"testing"
)

const epsilon = 0.5 // degrees — generous for float32 trig round-trips

func quatNear(a, b quat, eps float32) bool {
	// Quaternions q and -q represent the same rotation.
	d := a.dot(b)
	return d > 1-eps || d < -(1-eps)
}

func anglesNear(t *testing.T, gotR, gotP, gotY, wantR, wantP, wantY float32) {
	t.Helper()
	if math.Abs(float64(gotR-wantR)) > epsilon ||
		math.Abs(float64(gotP-wantP)) > epsilon ||
		math.Abs(float64(gotY-wantY)) > epsilon {
		t.Errorf("angles = (%.2f, %.2f, %.2f), want (%.2f, %.2f, %.2f)",
			gotR, gotP, gotY, wantR, wantP, wantY)
	}
}

func TestEulerToQuatIdentity(t *testing.T) {
	q := eulerToQuat(0, 0, 0)
	if !quatNear(q, quatIdentity(), 0.001) {
		t.Errorf("eulerToQuat(0,0,0) = %+v, want identity", q)
	}
}

func TestEulerRoundTrip(t *testing.T) {
	cases := []struct {
		roll, pitch, yaw float32
	}{
		{0, 0, 0},
		{45, -30, 90},
		{10, 20, 30},
		{-45, 60, -170},
		{0, 0, 180},
		{30, -10, 45},
	}
	for _, c := range cases {
		q := eulerToQuat(c.roll, c.pitch, c.yaw)
		r, p, y := q.toEuler()
		anglesNear(t, r, p, y, c.roll, c.pitch, c.yaw)
	}
}

func TestEulerRoundTripGimbalLock(t *testing.T) {
	// At pitch=90, roll and yaw are degenerate — but the orientation should match.
	q1 := eulerToQuat(0, 89.9, 0)
	r, p, y := q1.toEuler()
	q2 := eulerToQuat(r, p, y)
	if !quatNear(q1, q2, 0.001) {
		t.Errorf("gimbal lock round-trip failed: original %+v, reconstructed %+v", q1, q2)
	}
}

func TestSlerpEndpoints(t *testing.T) {
	q1 := eulerToQuat(10, 20, 30)
	q2 := eulerToQuat(40, -10, 90)

	start := q1.slerp(q2, 0)
	if !quatNear(start, q1, 0.001) {
		t.Errorf("slerp(q1,q2,0) = %+v, want %+v", start, q1)
	}

	end := q1.slerp(q2, 1)
	if !quatNear(end, q2, 0.001) {
		t.Errorf("slerp(q1,q2,1) = %+v, want %+v", end, q2)
	}
}

func TestSlerpIdentity(t *testing.T) {
	q := eulerToQuat(45, -30, 120)
	mid := q.slerp(q, 0.5)
	if !quatNear(mid, q, 0.001) {
		t.Errorf("slerp(q,q,0.5) = %+v, want %+v", mid, q)
	}
}

func TestSlerpMidpoint(t *testing.T) {
	q1 := eulerToQuat(0, 0, 0)
	q2 := eulerToQuat(0, 0, 90)
	mid := q1.slerp(q2, 0.5)
	_, _, yaw := mid.toEuler()
	if math.Abs(float64(yaw-45)) > epsilon {
		t.Errorf("slerp midpoint yaw = %.2f, want ~45", yaw)
	}
}

func TestSlerpShortestPath(t *testing.T) {
	q1 := eulerToQuat(0, 0, 0)
	// Create a quaternion and negate it — same rotation, opposite sign.
	q2 := eulerToQuat(0, 0, 10)
	q2neg := quat{-q2.W, -q2.X, -q2.Y, -q2.Z}

	// SLERP should still take the short path (result near q2, not far side).
	result := q1.slerp(q2neg, 0.5)
	_, _, yaw := result.toEuler()
	if math.Abs(float64(yaw-5)) > epsilon {
		t.Errorf("shortest path yaw = %.2f, want ~5", yaw)
	}
}

func TestNormalize(t *testing.T) {
	q := quat{2, 0, 0, 0}
	n := q.normalize()
	if !quatNear(n, quatIdentity(), 0.001) {
		t.Errorf("normalize({2,0,0,0}) = %+v, want identity", n)
	}

	// Zero quaternion returns identity.
	z := quat{}.normalize()
	if !quatNear(z, quatIdentity(), 0.001) {
		t.Errorf("normalize(zero) = %+v, want identity", z)
	}
}
