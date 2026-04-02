package wand

import (
	"net"
	"testing"
	"time"
)

func TestListenerLoopback(t *testing.T) {
	l := New(0) // port 0 = OS picks a free port
	if err := l.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop()

	// Get the actual port the listener bound to
	addr := l.conn.LocalAddr().(*net.UDPAddr)

	// Send a packet via loopback
	want := State{
		Roll: 10.0, Pitch: 20.0, Yaw: 30.0,
		AccelX: 1.0, AccelY: 2.0, AccelZ: 9.8,
		GyroX: 0.5, GyroY: -0.5, GyroZ: 0.0,
		Seq: 7,
	}
	data := EncodePacket(want)

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Wait for the listener to process it
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.PacketsReceived() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if l.PacketsReceived() == 0 {
		t.Fatal("no packets received within timeout")
	}

	got := l.State()
	if got.Seq != want.Seq {
		t.Errorf("Seq = %d, want %d", got.Seq, want.Seq)
	}
	if got.Roll != want.Roll {
		t.Errorf("Roll = %v, want %v", got.Roll, want.Roll)
	}
	if got.AccelZ != want.AccelZ {
		t.Errorf("AccelZ = %v, want %v", got.AccelZ, want.AccelZ)
	}
}

func TestListenerConnected(t *testing.T) {
	l := New(0)
	if err := l.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop()

	// No packets yet — should not be connected
	if l.Connected(time.Second) {
		t.Error("Connected() = true before any packets")
	}

	// Send a packet
	addr := l.conn.LocalAddr().(*net.UDPAddr)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer conn.Close()

	conn.Write(EncodePacket(State{Seq: 1}))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.Connected(time.Second) {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Connected() never became true after sending packet")
}

func TestListenerDropsInvalidPackets(t *testing.T) {
	l := New(0)
	if err := l.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop()

	addr := l.conn.LocalAddr().(*net.UDPAddr)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer conn.Close()

	// Send garbage
	conn.Write([]byte("not a wand packet"))

	time.Sleep(200 * time.Millisecond)

	if l.PacketsDropped() == 0 {
		t.Error("expected dropped packets for invalid data")
	}
	if l.PacketsReceived() != 0 {
		t.Error("expected zero received packets for invalid data")
	}
}
