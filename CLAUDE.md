# Wand

DIY toddler toy motion controller. A CodeCell C6 (ESP32-C6 + BNO085 9-axis IMU) sends orientation and motion data over WiFi UDP to a Go package that games can consume.

## Architecture

```
CodeCell C6 ──UDP 40Hz──> Go "wand" package ──> App (game selector + games)
```

- **Firmware** (`firmware/wand_controller/`): Arduino sketch. Reads IMU via CodeCell library, sends 40-byte UDP packets at 40Hz. Uses a discovery handshake (broadcast → ack) then unicasts to the discovered listener. Auto-recovers to discovery mode on client disconnect (ack timeout + send failure tracking + UDP socket reinit).
- **Go package** (root): `wand.New(port)` / `Start()` / `State()` / `Stop()`. Background goroutine reads UDP, stores latest state in `atomic.Value` for lock-free reads from the game loop.
- **App** (`cmd/play/`): Single entry point. Creates a shared wand listener, registers games, and runs the `App` wrapper which orchestrates an in-app game selector and game switching.
- **Games** (`games/`): `App` wrapper (`app.go`) implements `engine.Game` and delegates to a selector screen or the active game. `GameDef` registry in `games.go`. Individual games in sub-packages (e.g. `games/colorsphere/`). Games implement the `engine.Game` interface and receive the shared wand listener.
  - **Color Sphere** (`games/colorsphere/`): Inward-facing color sphere whose orientation is driven by wand Roll/Pitch/Yaw, plus an accelerometer-driven particle river. First game, for babies 10 months+.
  - **Selector** (`games/selector/`): Game selection screen shown at launch and when switching games.
- **Engine** (`pkg/`): Lightweight 3D engine built on SDL3 GPU API. Packages: `engine` (game loop), `renderer` (GPU pipelines, shaders, draw calls), `window` (SDL3 window + GPU device), `camera` (view/projection, frustum culling), `input` (keyboard/mouse), `mesh` (primitives), `ui` (pause menu), `settings` (JSON display settings), `sign` (pixel font), `shaders` (embedded compiled shaders for Metal/Vulkan/D3D12).
- **Protocol**: 40-byte data packets, little-endian. Magic `0x57 0x44`, version `0x01`, seq byte, then 9 float32s: roll/pitch/yaw (degrees), accel x/y/z (m/s²), gyro x/y/z (°/s). Also 4-byte control packets for discovery (`0x01`) and ack (`0x02`).

## Hardware

- **CodeCell C6**: ESP32-C6, BNO085 IMU, WiFi 6 + BLE 5, USB-C LiPo charging, Arduino-compatible
- CodeCell library API: `Init(MOTION_ROTATION_NO_MAG + MOTION_ACCELEROMETER + MOTION_GYRO)`, `Run(40)`, `Motion_RotationNoMagRead()`, `Motion_AccelerometerRead()`, `Motion_GyroRead()`

## Commands

```
make play                  # run the app (game selector + games)
make monitor               # display live IMU readings in terminal
make compile               # compile firmware
make upload                # compile + flash firmware to device
make test                  # run all Go tests
make sim                   # send fake IMU data (no hardware needed)
```

## Integration

The root wand package (UDP listener) has zero external dependencies. The `pkg/` engine packages depend on SDL3 and mathgl. To add a new game: create a package under `games/`, implement `engine.Game`, add a `GameDef` entry in `cmd/play/main.go`. The shared wand listener is passed to each game's constructor.
