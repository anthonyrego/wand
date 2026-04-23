// wandsim sends simulated wand IMU packets over UDP at 50Hz.
// Use this to test the wand package without hardware.
//
// Usage: go run ./cmd/wandsim [host:port]
// Default target: 127.0.0.1:9999
package main

import (
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"github.com/anthonyrego/wand"
)

func main() {
	target := "127.0.0.1:9999"
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve %s: %v\n", target, err)
		os.Exit(1)
	}

	// Use ListenUDP so we can both send discovery and receive acks
	conn, err := net.ListenUDP("udp", &net.UDPAddr{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Discovery: send discovery packets until we get an ack
	fmt.Printf("Discovering listener at %s...\n", target)
	discovery := wand.EncodeDiscovery()
	buf := make([]byte, 16)

	for {
		if _, err := conn.WriteToUDP(discovery, addr); err != nil {
			fmt.Fprintf(os.Stderr, "send discovery: %v\n", err)
			os.Exit(1)
		}

		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue // timeout, retry
		}

		pt, err := wand.ParseControlPacket(buf[:n])
		if err == nil && pt == wand.PacketTypeAck {
			fmt.Println("Listener found! Streaming data...")
			break
		}
	}

	// Clear deadline for the data loop
	conn.SetReadDeadline(time.Time{})

	fmt.Printf("Sending simulated wand data to %s at 50Hz\n", target)
	fmt.Println("Press Ctrl+C to stop")

	ticker := time.NewTicker(20 * time.Millisecond) // 50Hz
	defer ticker.Stop()

	var seq uint8
	start := time.Now()

	for range ticker.C {
		t := time.Since(start).Seconds()

		// Compose a smooth figure-8 orientation from three axis-angle quats.
		// Rotations are about the wand body frame: +X=tip, +Y=up, +Z=right.
		qRoll := quatAxisAngle(0, 0, 1, 0.5*math.Sin(t*0.8))  // roll around +Z (right)
		qPitch := quatAxisAngle(1, 0, 0, 0.35*math.Sin(t*0.6)) // pitch around +X (tip)
		qYaw := quatAxisAngle(0, 1, 0, math.Pi+0.5*math.Sin(t*0.3)) // yaw around +Y (up)
		q := qYaw.Mul(qPitch).Mul(qRoll).Normalize()

		s := wand.State{
			Q:         q,
			LinAccelX: float32(0.5 * math.Cos(t*0.8)),
			LinAccelY: float32(0.3 * math.Sin(t*1.2)),
			LinAccelZ: float32(0.5 * math.Sin(t*0.8)),
			GyroX:     float32(24.0 * math.Cos(t*0.8)),
			GyroY:     float32(12.0 * math.Cos(t*0.6)),
			GyroZ:     float32(27.0 * math.Cos(t*0.3)),
			Seq:       seq,
		}

		data := wand.EncodePacket(s)
		if _, err := conn.WriteToUDP(data, addr); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
		}

		seq++
	}
}

// quatAxisAngle builds a unit quaternion from an axis (x, y, z) and an
// angle in radians. The axis is normalized.
func quatAxisAngle(x, y, z, angle float64) wand.Quat {
	n := math.Sqrt(x*x + y*y + z*z)
	if n == 0 {
		return wand.QuatIdent()
	}
	s := math.Sin(angle/2) / n
	return wand.Quat{
		W: float32(math.Cos(angle / 2)),
		X: float32(x * s),
		Y: float32(y * s),
		Z: float32(z * s),
	}
}
