# Wand

A Go package that receives orientation and motion data from a wireless motion controller over UDP. Built for a DIY toddler toy using a [CodeCell C6](https://microbots.io/products/codecell-c6) (ESP32-C6 + BNO085 9-axis IMU).

## Usage

```go
package main

import (
    "fmt"
    "time"

    "github.com/anthonyrego/wand"
)

func main() {
    w := wand.New(9999)
    w.Start()
    defer w.Stop()

    for {
        s := w.State()
        fmt.Printf("Roll: %.1f  Pitch: %.1f  Yaw: %.1f\n", s.Roll, s.Pitch, s.Yaw)
        time.Sleep(20 * time.Millisecond)
    }
}
```

## How It Works

The CodeCell C6 reads its onboard IMU and broadcasts 40-byte UDP packets at 50Hz over WiFi. The Go package listens for these packets and provides lock-free, thread-safe access to the latest state from any goroutine.

```
CodeCell C6 ──UDP 50Hz──> Go "wand" package ──> Your game
```

### State

Each reading contains:
- **Rotation**: roll, pitch, yaw (degrees)
- **Acceleration**: x, y, z (m/s²)
- **Gyroscope**: x, y, z (°/s)

## Tools

Test without hardware using the included simulator:

```sh
# Terminal 1: listen for wand data
go run ./cmd/wandtest

# Terminal 2: send simulated IMU data at 50Hz
go run ./cmd/wandsim
```

## Hardware Setup

1. Flash `firmware/wand_controller/wand_controller.ino` to a CodeCell C6 using the Arduino IDE
2. Set your WiFi SSID and password in the sketch
3. The controller broadcasts to UDP port 9999 on your local network — no IP configuration needed

## Requirements

- Go 1.24+
- [CodeCell C6](https://microbots.io/products/codecell-c6) with the [CodeCell Arduino library](https://github.com/microbotsio/CodeCell)
