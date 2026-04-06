# Wand

DIY toddler toy motion controller. A CodeCell C6 (ESP32-C6 + BNO085 9-axis IMU) sends orientation and motion data over WiFi UDP to a Go package that games can consume.

## Architecture

```
CodeCell C6 ──UDP 40Hz──> Go "wand" package ──> Game (e.g. cmd/wandview)
```

- **Firmware** (`firmware/wand_controller/`): Arduino sketch. Reads IMU via CodeCell library, sends 40-byte UDP packets at 40Hz. Uses a discovery handshake (broadcast → ack) then unicasts to the discovered listener. Auto-recovers to discovery mode on client disconnect (ack timeout + send failure tracking + UDP socket reinit).
- **Go package** (root): `wand.New(port)` / `Start()` / `State()` / `Stop()`. Background goroutine reads UDP, stores latest state in `atomic.Value` for lock-free reads from the game loop.
- **Viewer** (`cmd/wandview/`): 3D color sphere viewer using the engine packages in `pkg/`. Renders an inward-facing color sphere whose orientation is driven by the wand's Roll/Pitch/Yaw, plus an accelerometer waveform overlay. Includes a pause menu with display settings.
- **Engine** (`pkg/`): Lightweight 3D engine built on SDL3 GPU API. Packages: `engine` (game loop), `renderer` (GPU pipelines, shaders, draw calls), `window` (SDL3 window + GPU device), `camera` (view/projection, frustum culling), `input` (keyboard/mouse), `mesh` (primitives), `ui` (pause menu), `settings` (JSON display settings), `sign` (pixel font), `shaders` (embedded compiled shaders for Metal/Vulkan/D3D12).
- **Protocol**: 40-byte data packets, little-endian. Magic `0x57 0x44`, version `0x01`, seq byte, then 9 float32s: roll/pitch/yaw (degrees), accel x/y/z (m/s²), gyro x/y/z (°/s). Also 4-byte control packets for discovery (`0x01`) and ack (`0x02`).

## Hardware

- **CodeCell C6**: ESP32-C6, BNO085 IMU, WiFi 6 + BLE 5, USB-C LiPo charging, Arduino-compatible
- CodeCell library API: `Init(MOTION_ROTATION_NO_MAG + MOTION_ACCELEROMETER + MOTION_GYRO)`, `Run(40)`, `Motion_RotationNoMagRead()`, `Motion_AccelerometerRead()`, `Motion_GyroRead()`

## Commands

```
make view                  # run 3D viewer (requires wand hardware)
make monitor               # display live IMU readings in terminal
make compile               # compile firmware
make upload                # compile + flash firmware to device
make test                  # run all Go tests
make sim                   # send fake IMU data (no hardware needed)
```

## Integration

The root wand package (UDP listener) has zero external dependencies. The `pkg/` engine packages depend on SDL3 and mathgl. A game using the engine would call `wand.New(9999)` in `Init()` and `l.State()` in `Update()`.
