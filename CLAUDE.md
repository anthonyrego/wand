# Wand

DIY toddler toy motion controller. A CodeCell C6 (ESP32-C6 + BNO085 9-axis IMU) sends orientation and motion data over WiFi UDP to a Go package that games can consume.

## Architecture

```
CodeCell C6 ──UDP 40Hz──> Go "wand" package ──> App (game selector + games)
```

- **Firmware** (`firmware/wand_controller/`): Arduino sketch. Reads IMU via CodeCell library, applies a configurable wand-frame remap quaternion (`WAND_REMAP_*` in `config.h`), sends 44-byte UDP packets at 40Hz. Uses a discovery handshake (broadcast → ack) then unicasts to the discovered listener. Auto-recovers to discovery mode on client disconnect (ack timeout + send failure tracking + UDP socket reinit).
- **Vendored library** (`firmware/libraries/CodeCell/`): Local copy of the [CodeCell](https://github.com/microbotsio/CodeCell) Arduino library with a small patch adding `Motion_RotationNoMagVectorRead(w, x, y, z)` so we can read the no-mag game rotation quaternion directly (the upstream library only exposes it as Euler angles). See `PATCHES.md` in that directory. The Makefile's `arduino-cli` invocation uses `--libraries firmware/libraries` so this copy wins over any globally installed one.
- **Go package** (root): `wand.New(port)` / `Start()` / `State()` / `Stop()`. Background goroutine reads UDP, stores latest state in `atomic.Value` for lock-free reads from the game loop. Defines a minimal zero-dependency `wand.Quat` type; games convert to `mgl32.Quat` at the call site.
- **App** (`cmd/play/`): Single entry point. Creates a shared wand listener, registers games, and runs the `App` wrapper which orchestrates an in-app game selector and game switching.
- **Games** (`games/`): `App` wrapper (`app.go`) implements `engine.Game` and delegates to a selector screen or the active game. `GameDef` registry in `games.go`. Individual games in sub-packages (e.g. `games/colorsphere/`). Games implement the `engine.Game` interface and receive the shared wand listener. Games consume `State.Q` (quaternion) directly; **never** subtract two Euler triples to compute a rotation delta — use `q_rel = q_neutral.Conjugate().Mul(q_current)` (or the mgl32 equivalent) for a true body-frame delta.
  - **Color Sphere** (`games/colorsphere/`): Inward-facing color sphere whose orientation follows the wand. Captures a neutral quaternion on first frame; the sphere's rotation is the body-frame delta from neutral, so whatever pose the wand is in at game start becomes "zero." Plus a linear-accel-driven particle river. First game, for babies 10 months+.
  - **Flying** (`games/flying/`): Flight simulator. Captures a neutral quaternion on start; each frame computes the body-frame delta and uses the imaginary parts (`2 * qRel.{X,Y,Z}`) as pitch/roll/yaw rate inputs. No gimbal lock, no axis entanglement.
  - **Drum Circle** (`games/drumcircle/`): Uses linear accel magnitude for hit detection (gravity-free — no threshold hack) and rotates a body-frame tip vector (`Q.Rotate({1,0,0})`) against an icosahedron face map for note selection. Synthesizes tones through `pkg/audio`.
  - **Selector** (`games/selector/`): Game selection screen shown at launch and when switching games.
- **Engine** (`pkg/`): Lightweight 3D engine built on SDL3 GPU API. Packages: `engine` (game loop), `renderer` (GPU pipelines, shaders, draw calls, HDR post-processing), `window` (SDL3 window + GPU device), `camera` (view/projection, frustum culling), `input` (keyboard/mouse), `mesh` (primitives + texture loading), `audio` (SDL3 audio output device wrapper for procedural synthesis), `ui` (pause menu with settings), `settings` (JSON display settings), `sign` (pixel font), `shaders` (embedded compiled shaders for Metal/Vulkan/D3D12).
- **Protocol** (`protocol.go`): 44-byte data packets, little-endian. Magic `0x57 0x44`, version `0x02`, seq byte, then 10 float32s: quaternion w/x/y/z (unit, wand body frame), linear accel x/y/z (m/s², gravity-compensated), gyro x/y/z (°/s). Also 4-byte control packets for discovery (`0x01`) and ack (`0x02`). `State.Euler()` provides a ZYX decomposition for cosmetic uses only.

## Hardware

- **CodeCell C6**: ESP32-C6, BNO085 IMU, WiFi 6 + BLE 5, USB-C LiPo charging, Arduino-compatible
- CodeCell library API (used): `Init(MOTION_ROTATION_NO_MAG + MOTION_LINEAR_ACC + MOTION_GYRO)`, `Run(40)` (Hz), `Motion_RotationNoMagVectorRead(w,x,y,z)` (**locally patched** — see `firmware/libraries/CodeCell/PATCHES.md`), `Motion_LinearAccRead()`, `Motion_GyroRead()`

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
