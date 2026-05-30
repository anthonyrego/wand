package wand

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	PacketSize      = 44
	MagicByte0      = 0x57 // 'W'
	MagicByte1      = 0x44 // 'D'
	ProtocolVersion = 0x02

	// Control packet types (4-byte packets used for discovery handshake).
	ControlPacketSize   = 4
	PacketTypeDiscovery = 0x01
	PacketTypeAck       = 0x02
)

var (
	ErrPacketTooShort    = errors.New("wand: packet too short")
	ErrBadMagic          = errors.New("wand: invalid magic bytes")
	ErrBadVersion        = errors.New("wand: unsupported protocol version")
	ErrUnknownPacketType = errors.New("wand: unknown control packet type")
)

// Quat is a unit quaternion (w + xi + yj + zk). Zero-dependency by design —
// games convert to their own math library's quaternion type at the call site
// (e.g. mgl32.Quat{W: q.W, V: mgl32.Vec3{q.X, q.Y, q.Z}}).
type Quat struct {
	W, X, Y, Z float32
}

// QuatIdent returns the identity quaternion (no rotation).
func QuatIdent() Quat { return Quat{W: 1} }

// Normalize returns q scaled to unit length. Returns identity for the zero
// quaternion.
func (q Quat) Normalize() Quat {
	n := float32(math.Sqrt(float64(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)))
	if n == 0 {
		return QuatIdent()
	}
	inv := 1 / n
	return Quat{q.W * inv, q.X * inv, q.Y * inv, q.Z * inv}
}

// Mul returns q * r. Quaternion multiplication composes rotations; q * r
// means "rotate by r, then by q" (like matrix multiplication).
func (q Quat) Mul(r Quat) Quat {
	return Quat{
		W: q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
		X: q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		Y: q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		Z: q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
	}
}

// Conjugate returns the inverse rotation for a unit quaternion.
func (q Quat) Conjugate() Quat { return Quat{q.W, -q.X, -q.Y, -q.Z} }

// Dot returns the 4D dot product. Sign is used to detect when two unit
// quaternions are on opposite sides of the 4-sphere (q and -q represent the
// same rotation); a negative dot means the shorter interpolation path goes
// through -r.
func (q Quat) Dot(r Quat) float32 {
	return q.W*r.W + q.X*r.X + q.Y*r.Y + q.Z*r.Z
}

// State holds the latest IMU reading from the wand controller.
// Q is the wand-body orientation quaternion (identity == neutral pose, with
// +X tip-forward, +Y up, +Z right after the firmware-side axis remap).
// LinAccel is gravity-compensated acceleration in m/s² (≈0 when still).
// Gyro is body-frame angular velocity in °/s.
type State struct {
	Q                               Quat
	LinAccelX, LinAccelY, LinAccelZ float32
	GyroX, GyroY, GyroZ             float32
	Seq                             uint8
}

// Euler decomposes Q into roll, pitch, yaw (degrees) using ZYX (Tait-Bryan)
// convention. Provided for cosmetic uses only — color mapping, debug
// overlays. Never subtract two Euler triples to compute a rotation delta;
// use quaternion math (q_rel = q_a.Conjugate().Mul(q_b)) instead.
func (s State) Euler() (roll, pitch, yaw float32) {
	q := s.Q
	// roll (x-axis rotation)
	sinrCosp := 2 * (q.W*q.X + q.Y*q.Z)
	cosrCosp := 1 - 2*(q.X*q.X+q.Y*q.Y)
	roll = float32(math.Atan2(float64(sinrCosp), float64(cosrCosp)))

	// pitch (y-axis rotation)
	sinp := 2 * (q.W*q.Y - q.Z*q.X)
	if sinp >= 1 {
		pitch = float32(math.Pi / 2)
	} else if sinp <= -1 {
		pitch = float32(-math.Pi / 2)
	} else {
		pitch = float32(math.Asin(float64(sinp)))
	}

	// yaw (z-axis rotation)
	sinyCosp := 2 * (q.W*q.Z + q.X*q.Y)
	cosyCosp := 1 - 2*(q.Y*q.Y+q.Z*q.Z)
	yaw = float32(math.Atan2(float64(sinyCosp), float64(cosyCosp)))

	const rad2deg = float32(180.0 / math.Pi)
	return roll * rad2deg, pitch * rad2deg, yaw * rad2deg
}

// ParsePacket validates and decodes a 44-byte UDP packet into a State.
func ParsePacket(data []byte) (State, error) {
	if len(data) < PacketSize {
		return State{}, ErrPacketTooShort
	}
	if data[0] != MagicByte0 || data[1] != MagicByte1 {
		return State{}, ErrBadMagic
	}
	if data[2] != ProtocolVersion {
		return State{}, ErrBadVersion
	}

	s := State{Seq: data[3]}
	s.Q.W = math.Float32frombits(binary.LittleEndian.Uint32(data[4:8]))
	s.Q.X = math.Float32frombits(binary.LittleEndian.Uint32(data[8:12]))
	s.Q.Y = math.Float32frombits(binary.LittleEndian.Uint32(data[12:16]))
	s.Q.Z = math.Float32frombits(binary.LittleEndian.Uint32(data[16:20]))
	s.LinAccelX = math.Float32frombits(binary.LittleEndian.Uint32(data[20:24]))
	s.LinAccelY = math.Float32frombits(binary.LittleEndian.Uint32(data[24:28]))
	s.LinAccelZ = math.Float32frombits(binary.LittleEndian.Uint32(data[28:32]))
	s.GyroX = math.Float32frombits(binary.LittleEndian.Uint32(data[32:36]))
	s.GyroY = math.Float32frombits(binary.LittleEndian.Uint32(data[36:40]))
	s.GyroZ = math.Float32frombits(binary.LittleEndian.Uint32(data[40:44]))
	return s, nil
}

// EncodePacket builds a 44-byte UDP packet from a State.
func EncodePacket(s State) []byte {
	buf := make([]byte, PacketSize)
	buf[0] = MagicByte0
	buf[1] = MagicByte1
	buf[2] = ProtocolVersion
	buf[3] = s.Seq
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(s.Q.W))
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(s.Q.X))
	binary.LittleEndian.PutUint32(buf[12:16], math.Float32bits(s.Q.Y))
	binary.LittleEndian.PutUint32(buf[16:20], math.Float32bits(s.Q.Z))
	binary.LittleEndian.PutUint32(buf[20:24], math.Float32bits(s.LinAccelX))
	binary.LittleEndian.PutUint32(buf[24:28], math.Float32bits(s.LinAccelY))
	binary.LittleEndian.PutUint32(buf[28:32], math.Float32bits(s.LinAccelZ))
	binary.LittleEndian.PutUint32(buf[32:36], math.Float32bits(s.GyroX))
	binary.LittleEndian.PutUint32(buf[36:40], math.Float32bits(s.GyroY))
	binary.LittleEndian.PutUint32(buf[40:44], math.Float32bits(s.GyroZ))
	return buf
}

// EncodeDiscovery builds a 4-byte discovery packet.
func EncodeDiscovery() []byte {
	return []byte{MagicByte0, MagicByte1, ProtocolVersion, PacketTypeDiscovery}
}

// EncodeAck builds a 4-byte acknowledgement packet.
func EncodeAck() []byte {
	return []byte{MagicByte0, MagicByte1, ProtocolVersion, PacketTypeAck}
}

// ParseControlPacket validates a 4-byte control packet and returns the packet type.
func ParseControlPacket(data []byte) (uint8, error) {
	if len(data) < ControlPacketSize {
		return 0, ErrPacketTooShort
	}
	if data[0] != MagicByte0 || data[1] != MagicByte1 {
		return 0, ErrBadMagic
	}
	if data[2] != ProtocolVersion {
		return 0, ErrBadVersion
	}
	pt := data[3]
	if pt != PacketTypeDiscovery && pt != PacketTypeAck {
		return 0, ErrUnknownPacketType
	}
	return pt, nil
}
