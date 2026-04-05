package wand

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// Listener receives UDP packets from a wand controller and provides
// thread-safe access to the latest IMU state.
type Listener struct {
	port int
	conn *net.UDPConn
	done chan struct{}

	state      atomic.Value // stores State
	lastPacket atomic.Int64 // unix nano of last valid packet
	remoteAddr atomic.Value // stores net.Addr

	// Stats
	packetsReceived     atomic.Uint64
	packetsDropped      atomic.Uint64
	discoveriesReceived atomic.Uint64

	lastAckSent atomic.Int64 // unix nano of last ack sent
}

// New creates a Listener that will bind to the given UDP port.
func New(port int) *Listener {
	l := &Listener{
		port: port,
		done: make(chan struct{}),
	}
	l.state.Store(State{})
	return l
}

// Start binds to the UDP port and begins reading packets in a background goroutine.
func (l *Listener) Start() error {
	addr := &net.UDPAddr{Port: l.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("wand: listen on port %d: %w", l.port, err)
	}
	l.conn = conn

	go l.readLoop()
	return nil
}

// Stop closes the UDP connection and stops the background goroutine.
func (l *Listener) Stop() {
	close(l.done)
	if l.conn != nil {
		l.conn.Close()
	}
}

// State returns the most recent IMU state. Lock-free and safe to call
// from any goroutine (e.g. game loop main thread).
func (l *Listener) State() State {
	return l.state.Load().(State)
}

// Connected returns true if a valid packet was received within the given timeout.
func (l *Listener) Connected(timeout time.Duration) bool {
	last := l.lastPacket.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < timeout
}

// RemoteAddr returns the address of the last sender, or nil if no packets received.
func (l *Listener) RemoteAddr() net.Addr {
	v := l.remoteAddr.Load()
	if v == nil {
		return nil
	}
	return v.(net.Addr)
}

// PacketsReceived returns the total number of valid packets received.
func (l *Listener) PacketsReceived() uint64 {
	return l.packetsReceived.Load()
}

// PacketsDropped returns the total number of invalid packets received.
func (l *Listener) PacketsDropped() uint64 {
	return l.packetsDropped.Load()
}

// DiscoveriesReceived returns the total number of discovery packets received.
func (l *Listener) DiscoveriesReceived() uint64 {
	return l.discoveriesReceived.Load()
}

func (l *Listener) readLoop() {
	buf := make([]byte, 128) // larger than PacketSize to detect oversized packets
	ack := EncodeAck()

	for {
		select {
		case <-l.done:
			return
		default:
		}

		l.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, addr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			continue // timeout or closed connection
		}

		switch {
		case n == ControlPacketSize:
			pt, err := ParseControlPacket(buf[:n])
			if err != nil {
				l.packetsDropped.Add(1)
				continue
			}
			if pt == PacketTypeDiscovery {
				l.discoveriesReceived.Add(1)
				l.conn.WriteToUDP(ack, addr)
				l.lastAckSent.Store(time.Now().UnixNano())
			}

		case n >= PacketSize:
			state, err := ParsePacket(buf[:n])
			if err != nil {
				l.packetsDropped.Add(1)
				continue
			}

			now := time.Now()
			l.state.Store(state)
			l.lastPacket.Store(now.UnixNano())
			l.remoteAddr.Store(addr)
			l.packetsReceived.Add(1)

			// Send periodic keepalive ack every 2 seconds
			if now.Sub(time.Unix(0, l.lastAckSent.Load())) > 2*time.Second {
				l.conn.WriteToUDP(ack, addr)
				l.lastAckSent.Store(now.UnixNano())
			}

		default:
			l.packetsDropped.Add(1)
		}
	}
}
