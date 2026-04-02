// wandtest displays live wand IMU data in the terminal.
//
// Usage: go run ./cmd/wandtest [port]
// Default port: 9999
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/anthonyrego/wand"
)

func main() {
	port := 9999
	if len(os.Args) > 1 {
		p, err := strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid port: %s\n", os.Args[1])
			os.Exit(1)
		}
		port = p
	}

	l := wand.New(port)
	if err := l.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(1)
	}
	defer l.Stop()

	fmt.Printf("Listening for wand on UDP port %d...\n", port)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	ticker := time.NewTicker(100 * time.Millisecond) // refresh at 10Hz
	defer ticker.Stop()

	for range ticker.C {
		s := l.State()
		connected := l.Connected(500 * time.Millisecond)

		status := "\033[31mDISCONNECTED\033[0m"
		if connected {
			remote := l.RemoteAddr()
			status = fmt.Sprintf("\033[32mCONNECTED\033[0m (%v)", remote)
		}

		// Move cursor up 8 lines and overwrite
		fmt.Print("\033[8A\033[J")
		fmt.Printf("  Status:  %s\n", status)
		fmt.Printf("  Packets: %d received, %d dropped\n", l.PacketsReceived(), l.PacketsDropped())
		fmt.Println()
		fmt.Printf("  Roll:  %+8.2f°    Accel X: %+7.3f m/s²    Gyro X: %+8.2f°/s\n", s.Roll, s.AccelX, s.GyroX)
		fmt.Printf("  Pitch: %+8.2f°    Accel Y: %+7.3f m/s²    Gyro Y: %+8.2f°/s\n", s.Pitch, s.AccelY, s.GyroY)
		fmt.Printf("  Yaw:   %+8.2f°    Accel Z: %+7.3f m/s²    Gyro Z: %+8.2f°/s\n", s.Yaw, s.AccelZ, s.GyroZ)
		fmt.Println()
		fmt.Printf("  Seq: %d\n", s.Seq)
	}
}
