package wand

import (
	"math"
	"testing"
)

func TestParsePacketRoundTrip(t *testing.T) {
	want := State{
		Q:         Quat{W: 0.707, X: 0.0, Y: 0.707, Z: 0.0},
		LinAccelX: 0.1, LinAccelY: -0.5, LinAccelZ: 0.25,
		GyroX: 10.0, GyroY: -5.0, GyroZ: 0.0,
		Seq:   42,
	}

	data := EncodePacket(want)
	got, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}

	if got.Seq != want.Seq {
		t.Errorf("Seq = %d, want %d", got.Seq, want.Seq)
	}

	fields := []struct {
		name      string
		got, want float32
	}{
		{"Q.W", got.Q.W, want.Q.W},
		{"Q.X", got.Q.X, want.Q.X},
		{"Q.Y", got.Q.Y, want.Q.Y},
		{"Q.Z", got.Q.Z, want.Q.Z},
		{"LinAccelX", got.LinAccelX, want.LinAccelX},
		{"LinAccelY", got.LinAccelY, want.LinAccelY},
		{"LinAccelZ", got.LinAccelZ, want.LinAccelZ},
		{"GyroX", got.GyroX, want.GyroX},
		{"GyroY", got.GyroY, want.GyroY},
		{"GyroZ", got.GyroZ, want.GyroZ},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("%s = %v, want %v", f.name, f.got, f.want)
		}
	}
}

func TestParsePacketTooShort(t *testing.T) {
	_, err := ParsePacket(make([]byte, 10))
	if err != ErrPacketTooShort {
		t.Errorf("got %v, want ErrPacketTooShort", err)
	}
}

func TestParsePacketBadMagic(t *testing.T) {
	data := EncodePacket(State{})
	data[0] = 0xFF
	_, err := ParsePacket(data)
	if err != ErrBadMagic {
		t.Errorf("got %v, want ErrBadMagic", err)
	}
}

func TestParsePacketBadVersion(t *testing.T) {
	data := EncodePacket(State{})
	data[2] = 0xFF
	_, err := ParsePacket(data)
	if err != ErrBadVersion {
		t.Errorf("got %v, want ErrBadVersion", err)
	}
}

func TestEncodePacketSize(t *testing.T) {
	data := EncodePacket(State{})
	if len(data) != PacketSize {
		t.Errorf("packet size = %d, want %d", len(data), PacketSize)
	}
}

func TestEncodeDiscovery(t *testing.T) {
	data := EncodeDiscovery()
	if len(data) != ControlPacketSize {
		t.Errorf("size = %d, want %d", len(data), ControlPacketSize)
	}
	if data[0] != MagicByte0 || data[1] != MagicByte1 {
		t.Error("bad magic bytes")
	}
	if data[2] != ProtocolVersion {
		t.Errorf("version = %d, want %d", data[2], ProtocolVersion)
	}
	if data[3] != PacketTypeDiscovery {
		t.Errorf("type = %d, want %d", data[3], PacketTypeDiscovery)
	}
}

func TestEncodeAck(t *testing.T) {
	data := EncodeAck()
	if len(data) != ControlPacketSize {
		t.Errorf("size = %d, want %d", len(data), ControlPacketSize)
	}
	if data[3] != PacketTypeAck {
		t.Errorf("type = %d, want %d", data[3], PacketTypeAck)
	}
}

func TestParseControlPacket(t *testing.T) {
	pt, err := ParseControlPacket(EncodeDiscovery())
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if pt != PacketTypeDiscovery {
		t.Errorf("type = %d, want %d", pt, PacketTypeDiscovery)
	}

	pt, err = ParseControlPacket(EncodeAck())
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if pt != PacketTypeAck {
		t.Errorf("type = %d, want %d", pt, PacketTypeAck)
	}

	_, err = ParseControlPacket([]byte{0x57, 0x44})
	if err != ErrPacketTooShort {
		t.Errorf("short: got %v, want ErrPacketTooShort", err)
	}

	_, err = ParseControlPacket([]byte{0xFF, 0x44, ProtocolVersion, 0x01})
	if err != ErrBadMagic {
		t.Errorf("magic: got %v, want ErrBadMagic", err)
	}

	_, err = ParseControlPacket([]byte{0x57, 0x44, 0xFF, 0x01})
	if err != ErrBadVersion {
		t.Errorf("version: got %v, want ErrBadVersion", err)
	}

	_, err = ParseControlPacket([]byte{0x57, 0x44, ProtocolVersion, 0xFF})
	if err != ErrUnknownPacketType {
		t.Errorf("unknown: got %v, want ErrUnknownPacketType", err)
	}
}

func TestQuatIdentityEuler(t *testing.T) {
	s := State{Q: QuatIdent()}
	roll, pitch, yaw := s.Euler()
	if math.Abs(float64(roll)) > 0.001 || math.Abs(float64(pitch)) > 0.001 || math.Abs(float64(yaw)) > 0.001 {
		t.Errorf("identity Euler = (%v, %v, %v), want (0, 0, 0)", roll, pitch, yaw)
	}
}

func TestQuatMulConjugate(t *testing.T) {
	// A 90° rotation about Y: w = cos(45°), y = sin(45°)
	c := float32(math.Cos(math.Pi / 4))
	sv := float32(math.Sin(math.Pi / 4))
	q := Quat{W: c, Y: sv}
	// q * q.Conjugate() should be identity.
	id := q.Mul(q.Conjugate())
	if math.Abs(float64(id.W-1)) > 1e-6 ||
		math.Abs(float64(id.X)) > 1e-6 ||
		math.Abs(float64(id.Y)) > 1e-6 ||
		math.Abs(float64(id.Z)) > 1e-6 {
		t.Errorf("q * conj(q) = %+v, want identity", id)
	}
}

func TestQuatNormalize(t *testing.T) {
	q := Quat{W: 2, X: 0, Y: 0, Z: 0}.Normalize()
	if math.Abs(float64(q.W-1)) > 1e-6 {
		t.Errorf("normalized = %+v, want W=1", q)
	}
	// zero -> identity
	z := Quat{}.Normalize()
	if z != (Quat{W: 1}) {
		t.Errorf("zero-normalized = %+v, want identity", z)
	}
}
