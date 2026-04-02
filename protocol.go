package wand

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	PacketSize      = 40
	MagicByte0      = 0x57 // 'W'
	MagicByte1      = 0x44 // 'D'
	ProtocolVersion = 0x01
)

var (
	ErrPacketTooShort  = errors.New("wand: packet too short")
	ErrBadMagic        = errors.New("wand: invalid magic bytes")
	ErrBadVersion      = errors.New("wand: unsupported protocol version")
)

// State holds the latest IMU readings from the wand controller.
type State struct {
	Roll, Pitch, Yaw       float32 // degrees
	AccelX, AccelY, AccelZ float32 // m/s²
	GyroX, GyroY, GyroZ   float32 // °/s
	Seq                    uint8
}

// ParsePacket validates and decodes a 40-byte UDP packet into a State.
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
	s.Roll = math.Float32frombits(binary.LittleEndian.Uint32(data[4:8]))
	s.Pitch = math.Float32frombits(binary.LittleEndian.Uint32(data[8:12]))
	s.Yaw = math.Float32frombits(binary.LittleEndian.Uint32(data[12:16]))
	s.AccelX = math.Float32frombits(binary.LittleEndian.Uint32(data[16:20]))
	s.AccelY = math.Float32frombits(binary.LittleEndian.Uint32(data[20:24]))
	s.AccelZ = math.Float32frombits(binary.LittleEndian.Uint32(data[24:28]))
	s.GyroX = math.Float32frombits(binary.LittleEndian.Uint32(data[28:32]))
	s.GyroY = math.Float32frombits(binary.LittleEndian.Uint32(data[32:36]))
	s.GyroZ = math.Float32frombits(binary.LittleEndian.Uint32(data[36:40]))
	return s, nil
}

// EncodePacket builds a 40-byte UDP packet from a State.
func EncodePacket(s State) []byte {
	buf := make([]byte, PacketSize)
	buf[0] = MagicByte0
	buf[1] = MagicByte1
	buf[2] = ProtocolVersion
	buf[3] = s.Seq
	binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(s.Roll))
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(s.Pitch))
	binary.LittleEndian.PutUint32(buf[12:16], math.Float32bits(s.Yaw))
	binary.LittleEndian.PutUint32(buf[16:20], math.Float32bits(s.AccelX))
	binary.LittleEndian.PutUint32(buf[20:24], math.Float32bits(s.AccelY))
	binary.LittleEndian.PutUint32(buf[24:28], math.Float32bits(s.AccelZ))
	binary.LittleEndian.PutUint32(buf[28:32], math.Float32bits(s.GyroX))
	binary.LittleEndian.PutUint32(buf[32:36], math.Float32bits(s.GyroY))
	binary.LittleEndian.PutUint32(buf[36:40], math.Float32bits(s.GyroZ))
	return buf
}
