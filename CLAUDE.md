# Wand

DIY toddler toy motion controller. A CodeCell C6 (ESP32-C6 + BNO085 9-axis IMU) sends orientation and motion data over WiFi UDP to a Go package that games can consume.

## Architecture

```
CodeCell C6 ──UDP 50Hz──> Go "wand" package ──> Game (e.g. Construct engine)
```

- **Firmware** (`firmware/wand_controller/`): Arduino sketch. Reads IMU via CodeCell library, broadcasts 40-byte UDP packets at 50Hz to 255.255.255.255:9999.
- **Go package** (root): `wand.New(port)` / `Start()` / `State()` / `Stop()`. Background goroutine reads UDP, stores latest state in `atomic.Value` for lock-free reads from the game loop.
- **Protocol**: 40 bytes, little-endian. Magic `0x57 0x44`, version `0x01`, seq byte, then 9 float32s: roll/pitch/yaw (degrees), accel x/y/z (m/s²), gyro x/y/z (°/s).

## Hardware

- **CodeCell C6**: ESP32-C6, BNO085 IMU, WiFi 6 + BLE 5, USB-C LiPo charging, Arduino-compatible
- CodeCell library API: `Init(MOTION_ROTATION_NO_MAG + MOTION_ACCELEROMETER + MOTION_GYRO)`, `Run(50)`, `Motion_RotationRead()`, `Motion_AccelerometerRead()`, `Motion_GyroRead()`

## Commands

```
go test ./...              # run all tests
go run ./cmd/wandsim       # send fake IMU data at 50Hz (no hardware needed)
go run ./cmd/wandtest      # display live IMU readings in terminal
```

## Integration

The package has zero external dependencies and no dependency on the Construct engine (`github.com/anthonyrego/construct`). A game using Construct would call `wand.New(9999)` in `Init()` and `l.State()` in `Update()`.
