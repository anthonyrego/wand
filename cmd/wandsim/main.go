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

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Sending simulated wand data to %s at 50Hz\n", target)
	fmt.Println("Press Ctrl+C to stop")

	ticker := time.NewTicker(20 * time.Millisecond) // 50Hz
	defer ticker.Stop()

	var seq uint8
	start := time.Now()

	for range ticker.C {
		t := time.Since(start).Seconds()

		// Smooth figure-8 motion pattern
		s := wand.State{
			Roll:   float32(30.0 * math.Sin(t*0.8)),
			Pitch:  float32(20.0 * math.Sin(t*0.6)),
			Yaw:    float32(180.0 + 90.0*math.Sin(t*0.3)),
			AccelX: float32(0.5 * math.Cos(t*0.8)),
			AccelY: float32(-9.8 + 0.3*math.Sin(t*1.2)),
			AccelZ: float32(0.5 * math.Sin(t*0.8)),
			GyroX:  float32(24.0 * math.Cos(t*0.8)),  // derivative of roll
			GyroY:  float32(12.0 * math.Cos(t*0.6)),  // derivative of pitch
			GyroZ:  float32(27.0 * math.Cos(t*0.3)),  // derivative of yaw
			Seq:    seq,
		}

		data := wand.EncodePacket(s)
		if _, err := conn.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
		}

		seq++
	}
}
