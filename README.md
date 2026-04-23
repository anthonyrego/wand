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
    w.SetSmoothing(0.3) // optional: quaternion nlerp smoothing [0, 1]
    w.Start()
    defer w.Stop()

    for {
        s := w.State()
        // s.Q is the wand-body orientation quaternion. Multiply by an
        // inverse of your captured "neutral" quaternion to get a
        // body-frame delta; never subtract Euler angles.
        fmt.Printf("Q: W=%.3f X=%.3f Y=%.3f Z=%.3f\n", s.Q.W, s.Q.X, s.Q.Y, s.Q.Z)
        time.Sleep(20 * time.Millisecond)
    }
}
```

## How It Works

The CodeCell C6 reads its onboard IMU and sends 44-byte UDP packets at 40Hz over WiFi. Connection is zero-config: the controller broadcasts a discovery packet, the Go listener responds with an ack, and the controller then unicasts directly to it. Periodic keepalive acks maintain the connection.

```
CodeCell C6 ──UDP 40Hz──> Go "wand" package ──> Your game
```

### State

Each reading contains:
- **Orientation**: unit quaternion (w, x, y, z) in the wand body frame (+X tip-forward, +Y up, +Z right). A `State.Euler()` helper decomposes it to roll/pitch/yaw degrees — but only use that for cosmetic display; never subtract two Euler triples to compute a rotation delta.
- **Linear acceleration**: x, y, z (m/s², gravity-compensated — reads ≈0 when still)
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

- **Color Sphere** — an inward-facing color sphere that rotates with the wand, plus a particle river driven by linear acceleration. Captures a neutral pose on the first frame, so however you're holding the wand when the game starts becomes the sphere's zero. A simple cause-and-effect toy for babies 10 months+.
- **Flying** — a flight simulator where the wand controls pitch, roll, and yaw as angular rates. Captures a neutral pose on the first frame and computes body-frame deltas from there, with particle effects and a sky dome.
- **Drum Circle** — point the wand in a direction and shake; each direction picks a note from a 20-note minor pentatonic mapped to the faces of an icosahedron. Uses linear acceleration magnitude for hit detection and synthesizes tones through SDL3 audio.

The app uses a lightweight 3D engine included in `pkg/` (SDL3 GPU API, cross-platform Metal/Vulkan/D3D12).

## Hardware Setup

1. Copy `firmware/wand_controller/config.h.example` to `config.h` and set your WiFi SSID and password
2. Run `make upload` — it will list connected boards and prompt you to select one before compiling and flashing
3. The controller discovers the listener automatically over UDP — no IP configuration needed

### Wand-frame calibration

`config.h` includes a `WAND_REMAP_*` quaternion that aligns the BNO085 sensor frame with the wand's physical axes (+X tip-forward, +Y up, +Z right). It defaults to identity. To calibrate:

1. Flash once with identity remap and run `make monitor`.
2. Hold the wand in its intended neutral pose (tip forward, top up).
3. Read the displayed `Quat` values.
4. Set `WAND_REMAP_W/X/Y/Z` to the inverse of that reading (same W, negated X/Y/Z).
5. Reflash. The neutral pose should now read as identity (W≈1, others≈0).

## Enclosure

The `models/` directory contains 3D-printable STL files and OpenSCAD sources for the wand enclosure (handle, star front/back, housing, module mount).

## Requirements

- Go 1.24+
- [CodeCell C6](https://microbots.io/products/codecell-c6). The [CodeCell Arduino library](https://github.com/microbotsio/CodeCell) is vendored in `firmware/libraries/CodeCell/` with a small local patch (see `PATCHES.md` there) that exposes the no-mag game rotation quaternion.
- [arduino-cli](https://arduino.github.io/arduino-cli/) (for firmware compilation and upload)
- Python 3 (used by the board selection script)
