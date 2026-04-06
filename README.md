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
    w.SetSmoothing(0.3) // optional: SLERP-based orientation smoothing [0, 1]
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

The CodeCell C6 reads its onboard IMU and sends 40-byte UDP packets at 40Hz over WiFi. Connection is zero-config: the controller broadcasts a discovery packet, the Go listener responds with an ack, and the controller then unicasts directly to it. Periodic keepalive acks maintain the connection.

```
CodeCell C6 ──UDP 40Hz──> Go "wand" package ──> Your game
```

### State

Each reading contains:
- **Rotation**: roll, pitch, yaw (degrees)
- **Acceleration**: x, y, z (m/s²)
- **Gyroscope**: x, y, z (°/s)

## Tools

```sh
make play       # launch the app (game selector + games)
make sim        # send fake IMU data over UDP (no hardware needed)
make monitor    # display live IMU readings in terminal
make test       # run all Go tests
make compile    # compile firmware
make upload     # compile + flash firmware (interactive board selection)
```

Test without hardware by running `make sim` in one terminal and `make play` in another.

### Games

`make play` launches a single app with an in-app game selector. The wand connects once and is shared across all games. Current games:

- **Color Sphere** — an inward-facing color sphere driven by the wand's orientation, with an accelerometer-driven particle river. A simple cause-and-effect toy for babies 10 months+.

The app uses a lightweight 3D engine included in `pkg/` (SDL3 GPU API, cross-platform Metal/Vulkan/D3D12).

## Hardware Setup

1. Copy `firmware/wand_controller/config.h.example` to `config.h` and set your WiFi SSID and password
2. Run `make upload` — it will list connected boards and prompt you to select one before compiling and flashing
3. The controller discovers the listener automatically over UDP — no IP configuration needed

## Enclosure

The `models/` directory contains 3D-printable STL files (handle, star front/back, full assembly) and the OpenSCAD source.

## Requirements

- Go 1.24+
- [CodeCell C6](https://microbots.io/products/codecell-c6) with the [CodeCell Arduino library](https://github.com/microbotsio/CodeCell)
- [arduino-cli](https://arduino.github.io/arduino-cli/) (for firmware compilation and upload)
- Python 3 (used by the board selection script)
