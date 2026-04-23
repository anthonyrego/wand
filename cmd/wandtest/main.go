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

		roll, pitch, yaw := s.Euler()

		// Move cursor up 11 lines and overwrite
		fmt.Print("\033[11A\033[J")
		fmt.Printf("  Status:  %s\n", status)
		fmt.Printf("  Packets: %d received, %d dropped\n", l.PacketsReceived(), l.PacketsDropped())
		fmt.Println()
		fmt.Printf("  Quat:  W=%+6.3f  X=%+6.3f  Y=%+6.3f  Z=%+6.3f\n", s.Q.W, s.Q.X, s.Q.Y, s.Q.Z)
		fmt.Printf("  Euler: Roll=%+7.2f°  Pitch=%+7.2f°  Yaw=%+7.2f°\n", roll, pitch, yaw)
		fmt.Println()
		fmt.Printf("  LinAccel X: %+7.3f m/s²    Gyro X: %+8.2f°/s\n", s.LinAccelX, s.GyroX)
		fmt.Printf("  LinAccel Y: %+7.3f m/s²    Gyro Y: %+8.2f°/s\n", s.LinAccelY, s.GyroY)
		fmt.Printf("  LinAccel Z: %+7.3f m/s²    Gyro Z: %+8.2f°/s\n", s.LinAccelZ, s.GyroZ)
		fmt.Println()
		fmt.Printf("  Seq: %d\n", s.Seq)
	}
}
