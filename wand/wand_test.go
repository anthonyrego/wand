package wand

import (
	"math"
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

	addr := l.conn.LocalAddr().(*net.UDPAddr)

	want := State{
		Q:         Quat{W: 0.707, X: 0.0, Y: 0.707, Z: 0.0},
		LinAccelX: 1.0, LinAccelY: 2.0, LinAccelZ: 0.5,
		GyroX: 0.5, GyroY: -0.5, GyroZ: 0.0,
		Seq:   7,
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
	if got.Q != want.Q {
		t.Errorf("Q = %+v, want %+v", got.Q, want.Q)
	}
	if got.LinAccelZ != want.LinAccelZ {
		t.Errorf("LinAccelZ = %v, want %v", got.LinAccelZ, want.LinAccelZ)
	}
}

func TestListenerConnected(t *testing.T) {
	l := New(0)
	if err := l.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop()

	if l.Connected(time.Second) {
		t.Error("Connected() = true before any packets")
	}

	addr := l.conn.LocalAddr().(*net.UDPAddr)
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer conn.Close()

	conn.Write(EncodePacket(State{Q: QuatIdent(), Seq: 1}))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.Connected(time.Second) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Connected() never became true after sending packet")
}

func TestListenerDiscovery(t *testing.T) {
	l := New(0)
	if err := l.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop()

	boundAddr := l.conn.LocalAddr().(*net.UDPAddr)
	addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: boundAddr.Port}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	defer conn.Close()

	if _, err := conn.WriteToUDP(EncodeDiscovery(), addr); err != nil {
		t.Fatalf("send discovery: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 16)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}

	pt, err := ParseControlPacket(buf[:n])
	if err != nil {
		t.Fatalf("parse ack: %v", err)
	}
	if pt != PacketTypeAck {
		t.Errorf("type = %d, want %d", pt, PacketTypeAck)
	}

	if l.DiscoveriesReceived() == 0 {
		t.Error("expected discovery count > 0")
	}

	want := State{Q: Quat{W: 0.707, X: 0.0, Y: 0.0, Z: 0.707}, Seq: 1}
	if _, err := conn.WriteToUDP(EncodePacket(want), addr); err != nil {
		t.Fatalf("send data: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.PacketsReceived() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := l.State()
	if got.Q != want.Q {
		t.Errorf("Q = %+v, want %+v", got.Q, want.Q)
	}
}

// TestListenerSmoothing sends an identity quat then a 180°-around-Y quat with
// smoothing=0.5, and expects a ~90°-around-Y result (NLERP midpoint).
func TestListenerSmoothing(t *testing.T) {
	l := New(0)
	l.SetSmoothing(0.5)
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

	// First packet: identity. Seeds the smoother.
	conn.Write(EncodePacket(State{Q: QuatIdent(), Seq: 1}))
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.PacketsReceived() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Second: 180° rotation around Y-axis (W=0, Y=1). After NLERP midpoint
	// we expect ~45° rotation around Y: W ≈ cos(45°), Y ≈ sin(45°).
	conn.Write(EncodePacket(State{Q: Quat{W: 0, X: 0, Y: 1, Z: 0}, Seq: 2}))
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.PacketsReceived() > 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := l.State()
	wantW := float32(math.Sqrt(2) / 2)
	wantY := float32(math.Sqrt(2) / 2)
	if math.Abs(float64(got.Q.W-wantW)) > 0.01 || math.Abs(float64(got.Q.Y-wantY)) > 0.01 {
		t.Errorf("smoothed Q = %+v, want ~{W:%.3f Y:%.3f}", got.Q, wantW, wantY)
	}
	if math.Abs(float64(got.Q.X)) > 0.01 || math.Abs(float64(got.Q.Z)) > 0.01 {
		t.Errorf("smoothed Q off-axis nonzero: %+v", got.Q)
	}
}

func TestListenerSmoothingZero(t *testing.T) {
	l := New(0) // default smoothing = 0
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

	want := State{Q: Quat{W: 0.707, X: 0.707, Y: 0, Z: 0}, Seq: 1}
	conn.Write(EncodePacket(want))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if l.PacketsReceived() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := l.State()
	if got.Q != want.Q {
		t.Errorf("raw Q = %+v, want %+v", got.Q, want.Q)
	}
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

	conn.Write([]byte("not a wand packet"))

	time.Sleep(200 * time.Millisecond)

	if l.PacketsDropped() == 0 {
		t.Error("expected dropped packets for invalid data")
	}
	if l.PacketsReceived() != 0 {
		t.Error("expected zero received packets for invalid data")
	}
}
