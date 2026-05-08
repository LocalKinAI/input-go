# input-go

Pure-Go mouse and keyboard synthesis for macOS.

Single binary, zero `cgo`, `go install`-able. Built on CGEvent (Quartz
Event Services) via `purego` + an embedded companion dylib — the same
pattern as [`sckit-go`](https://github.com/LocalKinAI/sckit-go) and
[`kinrec`](https://github.com/LocalKinAI/kinrec).

`input-go` is part of **KinKit** — a family of pure-Go macOS system
libraries powering the [LocalKin](https://localkin.ai) agent swarm.

```bash
go install github.com/LocalKinAI/input-go/cmd/input@latest
input click 400 300
input type "hello, world"
input hotkey cmd c
```

## Features

- **Mouse**: move, click (left/right/middle, N-clicks), drag with
  eased animation, smooth motion, scroll with momentum.
- **Keyboard**: press/release individual keys, type arbitrary UTF-8
  (including CJK + emoji — bypasses keyboard layout), hotkey combos
  with any modifier stack.
- **IME-safe paste** (v0.3): `PasteText` routes CJK / multibyte text
  through the macOS pasteboard + ⌘V instead of synthesizing one key
  event per character — Pinyin / Wubi / Kotoeri front-ends keep up.
  Original clipboard is restored after the paste settles.
- **Per-process routing** (v0.2): `WithPID(pid)` posts events directly
  to a target app via `CGEventPostToPid`, so the user's foreground
  window keeps focus while the agent operates a different app.
- **Defer-friendly modifier holding** (v0.2): `Hold(ctx, mods, keys)`
  returns a release closure designed for `defer` — eliminates the
  "stuck modifier" class of bug.
- **Geometry**: current cursor position, main screen size.
- **Permission**: probe / prompt for Accessibility trust.
- **No cgo**: downstream projects stay pure Go. The ObjC companion
  dylib is `//go:embed`ded and extracted to `~/Library/Caches` on
  first call.
- **Universal**: single Mach-O ships both `arm64` and `x86_64` (~60 KB).

## Install

```bash
# CLI
go install github.com/LocalKinAI/input-go/cmd/input@latest

# Library
go get github.com/LocalKinAI/input-go
```

Requires macOS 12+ and Go 1.22+.

## Permission

macOS requires the invoking binary to be listed in
**System Settings → Privacy & Security → Accessibility** for synthesized
events to take effect. Without permission, calls return `nil` (no
error) but events have no visible effect — this matches the silent
failure model of CGEvent itself.

```go
if !input.Trusted() {
    if !input.PromptTrust() {
        log.Fatal("accessibility permission denied — grant in System Settings")
    }
}
```

## Library usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/LocalKinAI/input-go"
)

func main() {
    ctx := context.Background()
    if err := input.Load(); err != nil {
        log.Fatal(err)
    }
    if err := input.RequireTrust(); err != nil {
        log.Fatal(err)
    }

    // Smoothly move cursor to (800, 400) over 300 ms
    input.MoveSmooth(ctx, 800, 400, 300*time.Millisecond)

    // Double-click there
    input.DoubleClick(ctx, 800, 400)

    // Type a string into the focused field (Latin-1 — fast, key-by-key)
    input.Type(ctx, "Hello world")

    // CJK / multibyte / IME-active app — paste through clipboard so
    // Pinyin / Wubi / Kotoeri don't drop characters at full keyboard
    // rate. v0.3 addition; original clipboard is restored after.
    input.PasteText(ctx, "你好世界 👋")

    // Cmd+Shift+T (reopen closed tab)
    input.Hotkey(ctx, input.ModCommand|input.ModShift, input.KeyT)

    // Drag from (100,100) to (500,100) over 200 ms
    input.Drag(ctx, 100, 100, 500, 100, 200*time.Millisecond)
}
```

## CLI usage

```bash
# Mouse
input move 400 300                          # jump cursor
input move 400 300 --smooth 300ms           # animate
input click                                 # click at current pos
input click 400 300 --button right          # right-click at pos
input click --count 2                       # double-click at current pos
input drag 100 100 500 100 --duration 250ms

# Scroll
input scroll 0 -200                         # scroll up 200 px
input scroll 0 -200 --smooth 500ms

# Keyboard
input type "hello world"
input type "混合内容 Hello 👋" --delay 30ms
input press enter
input press f5
input hotkey cmd c                          # copy
input hotkey cmd+shift t                    # reopen closed tab
input hotkey cmd+shift+3                    # screenshot

# Introspection
input cursor                                # prints "X Y"
input screen                                # prints "W H"
input trust                                 # prints 1 or 0
input trust --prompt                        # triggers system dialog
input version
```

## How it works

`input-go` follows the **embedded dylib pattern** documented in Paper #9
of [localkin.dev/papers](https://localkin.dev/papers).

```
Go code  ─── purego.Dlopen ────► libinput_sync.dylib (embedded)
                                     │
                                     └──► CGEvent* APIs
```

- `objc/input_events.m` — 200 LOC ObjC shim exposing 8 C-ABI functions
  (`input_mouse_event`, `input_key`, `input_type_unicode`, etc.).
- `internal/dylib/libinput_sync.dylib` — universal Mach-O, built by
  `make dylib`, committed to the repo so `go get` works without
  requiring downstream users to install `clang`.
- On first call, `Load()` extracts the embedded bytes to
  `~/Library/Caches/input-go/<content-hash>/libinput_sync.dylib` and
  Dlopens from there.

## Known limitations (current as of v0.3)

- **macOS only.** Linux/X11 and Windows/WinAPI would need sibling
  backends; out of scope.
- **No keyboard event capture** — input-go synthesizes events, it
  doesn't listen for them. KinClaw uses
  [`kinax-go`](https://github.com/LocalKinAI/kinax-go)'s `Observer`
  (push-based AX events) for the agent-observes-human use case
  instead, since AX gives semantic notifications (`AXFocusedUIElement
  Changed`, `AXValueChanged`) without polling. A raw-event
  `input-go/listen` is therefore deferred indefinitely — file an
  issue if you need it.
- **Unicode typing bypasses keyboard layout.** `input.Type("A")` emits
  a literal `A` character event, not `Shift+a`. Apps that watch raw
  keycodes (games, key-remappers) won't see a modifier press. Use
  `input.Hotkey(input.ModShift, input.KeyA)` if you need layout-aware
  behavior. For CJK / IME contexts, prefer `input.PasteText` (v0.3).
- **`PasteText` clobbers non-text clipboard.** The pasteboard restore
  step only handles plain text — image / file / RTF clipboards do
  not survive a PasteText round-trip. Documented at the call site.
- **CapsLock state is not tracked.** If the user has CapsLock engaged,
  typed letters will come out uppercase regardless of your intent.
- **Multi-display coordinate systems** are passed through unchanged —
  `(0, 0)` is the top-left of the main display; external displays live
  in their configured coordinate space.
- **Tested only on macOS 26.3 arm64** so far; Intel + macOS 14/15
  verification pending CI.

## Roadmap

- **v0.2** — `WithPID` background-safe input routing (per-process
  `CGEventPostToPid`) + `Hold` defer-friendly modifier scope.
  **Shipped 2026-04-28.**
- **v0.3** — `PasteText` IME-safe text injection via pasteboard +
  ⌘V + restore + `ReadClipboard` / `WriteClipboard` building blocks.
  **Shipped 2026-05-07.**
- **v0.4** (planned) — `WithDelay(d)` PostOption for screen demos /
  rate-limited apps; `WithEventTap` to pick session vs HID tap;
  `MouseDown` / `MouseUp` raw primitives for callers building their
  own gesture engines.
- **v0.5** (planned) — Raw IOKit HID path for games and apps that
  bypass CGEventTap (e.g. anti-cheat-protected games — won't work
  there even with HID, but we'll document the failure clearly).
- **Cross-platform backends** — only if a user files an issue asking.

## Contributing

```bash
git clone https://github.com/LocalKinAI/input-go
cd input-go
make dylib     # rebuild universal Mach-O after ObjC changes
make test      # unit tests (safe — no events synthesized)
make test-integration   # needs Accessibility permission, actually moves cursor
make lint      # go vet + staticcheck + golangci-lint
```

## License

MIT. See `LICENSE`.

## See also

- [`sckit-go`](https://github.com/LocalKinAI/sckit-go) — ScreenCaptureKit.
- [`kinrec`](https://github.com/LocalKinAI/kinrec) — screen + audio recorder.
- [Embedded Dylib paper](https://www.localkin.dev/papers/embedded-dylib)
  — the architectural pattern behind all KinKit libraries.
