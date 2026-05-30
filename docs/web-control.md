# TODO / Future: Full web control

**Status:** implemented. The unified `pkg/control` server (port 8080), the
boot/QR splash, web-driven game switching, per-game pages (flashcards + drum
circle), web video settings, Esc-×3 quit, and HDR removal are all in. Decisions
are recorded below for context.

## Vision

Move *all* control of the app to a web UI on the local network. The screen becomes
a pure display (the "TV"); a parent drives everything from a phone or laptop. On
launch the app shows nothing but a connect prompt:

```
        Connect to
   http://10.10.70.22:8080
```

From that one page you can:

- **Pick / switch the active game** (replaces the in-app selector screen).
- **Adjust video/display settings**: fullscreen, resolution, draw distance, HDR
  (currently in the in-app pause menu, `pkg/ui`).
- **Adjust per-game settings**: e.g. drum-circle hit thresholds/exponent, sphere
  colors, flying sensitivity — anything a game wants to expose.
- **Personalize** the wand name (already web-driven, see below).
- **Manage flashcards** (already web-driven, see below).

The keyboard becomes optional. The wand stays the in-game input device; the web
page is the "remote control" for everything around the game.

## What already exists (building blocks)

- `pkg/profile` — an **always-on** `net/http` server (owner name, port 8090) with
  an embedded SPA + REST and disk persistence (`~/.toybox/profile.json`). A revision
  counter lets the game loop pick up changes live without a restart. This is the
  closest template for the unified control server.
- `games/flashcard` — a web admin (port 8080) that runs **only while that game is
  active**: embedded SPA + REST, disk persistence, and a mutex-guarded store the
  game loop diffs each frame (all GPU work stays on the loop thread). Good model
  for the CPU-side store / loop-thread split.
- `games/selector` + `games/app.go` — in-app game selection and switching
  (`App.switchTo`). Web game-switching would set the same field from an HTTP
  handler instead of from keyboard input.
- `pkg/ui` pause menu — applies display settings at runtime via
  `engine.ApplyDisplaySettings(fs, w, h, rd, hdr)`. Note: these are **not**
  persisted today. `pkg/settings` has `Load`/`Save` to JSON but they're currently
  unused — wiring persistence in is part of this work.

## Decisions

These resolve the earlier open questions; build to these.

- **Discovery: LAN IP for now, plus a QR code.** The on-screen splash shows the
  LAN-IP URL (best-effort via `profile.LocalIP()`) *and* a QR code encoding it so a
  parent can open it from a phone without typing. No mDNS for now (revisit if DHCP
  churn becomes annoying).
- **Security: none.** Home LAN toddler toy; no auth. Server binds all interfaces —
  state the assumption in the README/server doc and move on.
- **Per-game pages.** Not one flat SPA. The control site is a shell (Games / Name /
  Video) plus a dedicated page per active game for that game's own settings, which
  are editable *while the game is running*. Flashcards is the existing precedent —
  its admin becomes the flashcard game page under the unified server.
- **Canonical port: 8080.** The unified always-on server lives on 8080 (the port
  parents may already know from the flashcard "open on your phone" flow); this is
  the address shown on the splash. `pkg/profile`'s 8090 server folds into it.
- **No auto-launch.** At boot the screen shows only the connect/QR splash — no game
  (not even last-played) runs until a parent picks one from the web shell. Choosing
  a game from the shell is the *only* way into a game; quitting back returns to the
  splash.
- **No keyboard control.** The keyboard is no longer an input/control surface at
  all. The *only* key handling left: pressing **Escape three times** quits the app,
  with on-screen feedback each press (e.g. "Press Esc 2 more times to quit…",
  counter resets after a short idle). Everything else is web-driven.
- **Video settings: apply on save.** No live-drag preview; the page POSTs a full
  settings payload and the loop applies it on the next frame via
  `ApplyDisplaySettings`. Persist via `pkg/settings`.
- **Remove HDR.** Drop HDR as an exposed option; it stays off. Remove it from the
  settings UI/schema now; rip out the post-processing feature code in a later pass.

## Sketch of the work

1. **One control server, always on.** Promote a single server (generalize
   `pkg/profile`'s, or a new `pkg/control`) started in `cmd/play/main.go` for the
   app's whole life. The flashcard admin folds in as the flashcard game's per-game
   page. Canonical port **8080** (the on-screen URL); `pkg/profile`'s 8090 server
   folds into it.
2. **Command channel, loop-thread-safe.** Web handlers must not touch SDL/GPU.
   Mirror the flashcard pattern: handlers mutate a mutex-guarded state + bump a
   revision; the game loop reads snapshots each frame and performs the actual
   switch / `ApplyDisplaySettings` / GPU work on its own thread.
3. **Game registry exposed over REST.** `GameDef` list → `GET /api/games`;
   `POST /api/current {id}` drives `App.switchTo`. The in-app selector screen is
   replaced by the connect/QR splash — game choice moves entirely to the web shell.
4. **Per-game settings, advertised + persisted.** Each game advertises its tunables
   (a small descriptor) so its page renders controls generically and edits apply
   live while it runs. Display settings use the same get/set + `pkg/settings`
   persistence, applied on save. (No HDR field.)
5. **Boot/idle splash.** The launch screen is the connect URL + QR code (reuse the
   flashcard "OPEN ON YOUR PHONE" overlay style). Esc-×3-to-quit feedback overlays
   on top of whatever is showing.

## Notes / things to verify during build

- **`ApplyDisplaySettings` mid-frame.** Resolution changes recreate the swapchain;
  it's already centralized there, but confirm it's safe to trigger from a revision
  diff between frames (apply-on-save makes this once-per-save, not per-drag).
- **Esc-×3 ergonomics.** Pick the reset window (e.g. ~1.5s of no Esc resets the
  count) and where the counter renders so it's visible over any game/splash.
