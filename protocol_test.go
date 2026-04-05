package wand

import (
	"math"
	"testing"
)

func TestParsePacketRoundTrip(t *testing.T) {
	want := State{
		Roll: 45.5, Pitch: -12.3, Yaw: 180.0,
		AccelX: 0.1, AccelY: -9.8, AccelZ: 0.5,
		GyroX: 10.0, GyroY: -5.0, GyroZ: 0.0,
		Seq: 42,
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
		name     string
		got, want float32
	}{
		{"Roll", got.Roll, want.Roll},
		{"Pitch", got.Pitch, want.Pitch},
		{"Yaw", got.Yaw, want.Yaw},
		{"AccelX", got.AccelX, want.AccelX},
		{"AccelY", got.AccelY, want.AccelY},
		{"AccelZ", got.AccelZ, want.AccelZ},
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
	// Valid discovery
	pt, err := ParseControlPacket(EncodeDiscovery())
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if pt != PacketTypeDiscovery {
		t.Errorf("type = %d, want %d", pt, PacketTypeDiscovery)
	}

	// Valid ack
	pt, err = ParseControlPacket(EncodeAck())
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if pt != PacketTypeAck {
		t.Errorf("type = %d, want %d", pt, PacketTypeAck)
	}

	// Too short
	_, err = ParseControlPacket([]byte{0x57, 0x44})
	if err != ErrPacketTooShort {
		t.Errorf("short: got %v, want ErrPacketTooShort", err)
	}

	// Bad magic
	_, err = ParseControlPacket([]byte{0xFF, 0x44, 0x01, 0x01})
	if err != ErrBadMagic {
		t.Errorf("magic: got %v, want ErrBadMagic", err)
	}

	// Bad version
	_, err = ParseControlPacket([]byte{0x57, 0x44, 0xFF, 0x01})
	if err != ErrBadVersion {
		t.Errorf("version: got %v, want ErrBadVersion", err)
	}

	// Unknown type
	_, err = ParseControlPacket([]byte{0x57, 0x44, 0x01, 0xFF})
	if err != ErrUnknownPacketType {
		t.Errorf("unknown: got %v, want ErrUnknownPacketType", err)
	}
}

func TestParsePacketSpecialFloats(t *testing.T) {
	s := State{Roll: 0, Pitch: -0, Yaw: math.Float32frombits(0)}
	data := EncodePacket(s)
	got, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}
	if got.Roll != 0 {
		t.Errorf("Roll = %v, want 0", got.Roll)
	}
}
