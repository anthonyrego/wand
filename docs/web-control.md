# TODO / Future: Full web control

**Status:** idea, not scheduled. Recorded for later — do not implement yet.

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
  an embedded SPA + REST and disk persistence (`~/.wand/profile.json`). A revision
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

## Sketch of the work

1. **One control server, always on.** Promote a single server (generalize
   `pkg/profile`'s, or a new `pkg/control`) started in `cmd/play/main.go` for the
   app's whole life. Decide whether the flashcard admin folds into it or stays a
   per-game sub-page it links to. Pick one canonical port (the on-screen URL).
2. **Command channel, loop-thread-safe.** Web handlers must not touch SDL/GPU.
   Mirror the flashcard pattern: handlers mutate a mutex-guarded state + bump a
   revision; the game loop reads snapshots each frame and performs the actual
   switch / `ApplyDisplaySettings` / GPU work on its own thread.
3. **Game registry exposed over REST.** `GameDef` list → `GET /api/games`;
   `POST /api/current {id}` drives `App.switchTo`. The on-screen selector becomes
   optional (keep as a fallback, or replace with a "connect to …" splash).
4. **Settings model + persistence.** Define a settings schema (display + per-game),
   serve get/set, and actually persist via `pkg/settings` (or an extension). Each
   game advertises its tunables (a small descriptor) so the page renders controls
   generically.
5. **Boot splash.** When no display interaction is expected, the launch screen is
   just the connect URL (reuse the flashcard "OPEN ON YOUR PHONE" overlay style).

## Open questions

- **Discovery / fixed address.** The on-screen URL uses the LAN IP (best-effort via
  `profile.LocalIP()`), which can change with DHCP. Consider mDNS (`wand.local`) or
  a QR code so the address is stable and easy to open.
- **Security.** It's a toddler toy on a home LAN, so likely no auth — but the server
  binds all interfaces. Note the assumption explicitly if we keep it open.
- **One page vs. per-game pages.** A single SPA with tabs (Games / Settings / Name /
  Flashcards) vs. linked sub-pages. Single SPA is probably cleaner for parents.
- **Keep keyboard as fallback?** Useful for dev and when no phone is handy. Probably
  yes, at least behind a flag.
- **Live vs. apply-on-save** for video settings (resolution changes recreate the
  swapchain — already handled by `ApplyDisplaySettings`, but confirm it's safe to
  trigger from a revision diff mid-frame).
