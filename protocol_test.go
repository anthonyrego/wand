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
