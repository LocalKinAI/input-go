# Changelog

All notable changes to input-go are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [SemVer](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-05-07

Adds **`PasteText` + clipboard helpers** — IME-safe text injection
via the macOS pasteboard. Lifted out of `kinclaw/pkg/skill` (where
it was a private helper) into the kit so every input-go consumer
can paste CJK / multibyte text without re-implementing the
pbcopy → ⌘V → restore dance.

### Added — `input.PasteText(ctx, text, opts...) error`

```go
// Plain ASCII typing — Type works fine.
input.Type(ctx, "hello world")

// CJK / IME territory — Type drops chars at full keyboard rate.
// PasteText routes through the system pasteboard instead.
input.PasteText(ctx, "你好世界")

// Same opts work — paste into a background app via PID.
input.PasteText(ctx, longChineseText, input.WithPID(targetPID))
```

PasteText:
1. Saves the user's current clipboard (text only — image / file /
   RTF do not survive the round-trip, by macOS pasteboard design).
2. Writes the new text via `pbcopy`.
3. Fires ⌘V via the existing `Hotkey` path so `WithPID` and other
   `PostOption`s route consistently with the rest of input-go.
4. Sleeps 150ms to let the target app read the pasteboard.
5. Restores the original clipboard (best-effort).

The IME-safety property is the headline: `Type` synthesizes one key
event per character at ~100 char/s, which Pinyin / Wubi / Kotoeri
front-ends frequently can't keep up with — characters get dropped or
the IME composition state desyncs. PasteText sidesteps the IME
entirely by routing through the system pasteboard.

### Added — `input.ReadClipboard` / `input.WriteClipboard`

```go
prev, _ := input.ReadClipboard(ctx)         // pbpaste wrapper
input.WriteClipboard(ctx, "queued payload") // pbcopy wrapper
```

Building blocks for code that wants the pasteboard without firing ⌘V.
Used by callers that batch multiple paste operations or need to
inspect / round-trip the clipboard without going through the full
PasteText flow.

### Why this matters

Bilingual / non-Latin agent users hit the IME-drop bug constantly —
the agent asks the model "type this Chinese sentence" and the user
sees half of it land. `PasteText` makes that case actually work,
without forcing every consumer to maintain their own pbcopy/pbpaste
scaffolding.

The trade-off: PasteText is ~200ms slower than `Type` (clipboard
write + ⌘V + 150ms settle + clipboard restore) and clobbers the
non-text portion of the user's clipboard. For Latin-1 typing, keep
using `Type`. For CJK / multibyte / IME-active typing, use
`PasteText`.

### Build

- Pure Go — no ObjC, no dylib rebuild. `pbcopy` / `pbpaste` are
  shelled out via `os/exec` (universal on macOS, no ABI risk).
- 4 new test cases covering pbcopy/pbpaste round-trip, restore
  semantics, and ⌘V keystroke wiring through `Hotkey`.

## [0.2.0] - 2026-04-28

Two improvements harvested from the cross-language survey done for
KinClaw 2026-04-28: **per-process input routing** (axcli's headline
"background-safe input" pattern) and a **`Hold` context-manager**
inspired by pyautogui's `with hold(key)` form.

Both land at the same time because they touch the same hot path —
the dylib's event-posting functions. The dylib's exported symbols
gained a leading `pid` parameter (breaking ABI for direct dylib
consumers, but transparent to callers using the Go API), and every
public Go function now accepts a variadic `opts ...PostOption` tail.
Existing source-level callers keep compiling unchanged.

### Added

#### `WithPID(pid)` — background-safe input via `CGEventPostToPid`

```go
input.Click(ctx, 400, 300, input.WithPID(int32(targetPID)))
input.Type(ctx, "hello", input.WithPID(int32(targetPID)))
input.Hotkey(ctx, input.ModCommand, input.KeyS, input.WithPID(int32(targetPID)))
```

Routes the synthesized event directly to the target process — the
focused app stays focused, your editor doesn't lose its insertion
point, and multi-window workflows finally work without focus-thrash.

Verified working on Lark / VSCode / Chrome and other Electron + Web
View hosts (the same lineup axcli proved). Some Apple sandboxed apps
(newer Mail / Messages) may ignore PID-targeted events and need the
default system-wide route — fall back by omitting the option.

`WithPID(0)` (or no option at all) preserves the legacy behavior:
post to `kCGHIDEventTap` system-wide, the focused app receives the
event and may come to front.

#### `Hold(ctx, mods, keys, opts...)` — defer-friendly modifier holding

```go
defer input.Hold(ctx, input.ModShift, nil)()
input.Press(ctx, input.KeyTab)   // Shift+Tab
input.Press(ctx, input.KeyTab)   // still Shift+Tab
```

Returns a release closure designed for `defer`: the modifier keys
get released on scope exit even if the surrounding code panics or
returns early. Eliminates the "stuck modifier" class of bug where
an error path between manual `KeyDown` / `KeyUp` calls leaves a
shift / cmd / option held at the OS level — every subsequent input
gets corrupted until the user notices and taps the modifier
themselves.

Multi-modifier scope works as you'd expect:
```go
defer input.Hold(ctx, input.ModCommand|input.ModShift, nil)()
input.Press(ctx, input.KeyT)     // Cmd+Shift+T (reopen closed tab)
```
Modifiers release in reverse order (Shift before Command). The
release closure is idempotent — calling it twice is safe but only
the first call has effect.

`keys` is a slice (not variadic) because the function already has a
variadic `opts` tail. Pass `nil` for the modifier-only case.

### Changed

#### Variadic `opts ...PostOption` on every event-posting func

The full list of touched signatures: `Move`, `MoveBy`, `MoveSmooth`,
`Click`, `ClickAt`, `ClickAtButton`, `DoubleClick`, `RightClick`,
`Drag`, `Scroll`, `ScrollSmooth`, `KeyDown`, `KeyUp`, `Press`,
`Hotkey`, `Type`, `TypeSlow` — each gained `opts ...PostOption`
at the end.

`PostOption` is the only new exported type. The only `PostOption`
constructor in v0.2.0 is `WithPID` — future options (e.g.
`WithEventTap` to pick session vs HID tap, `WithDelay` for demos)
can be added without further signature churn.

#### Dylib ABI — pid as the first argument

The four event-posting C symbols changed signature:

```c
// Before (v0.1.0)
int32_t input_mouse_event(double x, double y, int32_t button, int32_t kind, int32_t clicks);
int32_t input_scroll(int32_t dx, int32_t dy);
int32_t input_key(int32_t keycode, int32_t down, uint64_t flags);
int32_t input_type_unicode(const char *utf8, int32_t utf8_len);

// After (v0.2.0)
int32_t input_mouse_event(int32_t pid, double x, double y, ...);
int32_t input_scroll(int32_t pid, int32_t dx, int32_t dy);
int32_t input_key(int32_t pid, int32_t keycode, int32_t down, uint64_t flags);
int32_t input_type_unicode(int32_t pid, const char *utf8, int32_t utf8_len);
```

The Go side is updated in lockstep, and v0.2.0 ships the matching
embedded dylib. Direct dylib consumers (rare — purego is the
intended path) need to update their function prototypes.

`pid` semantics: `pid > 0` → `CGEventPostToPid`; `pid <= 0` →
`CGEventPost(kCGHIDEventTap, ...)` (legacy).

### Why this matters

`WithPID` is the v0.x line's biggest behavioral upgrade since the
initial release. Without it, every input synthesis steals focus from
the user's foreground app — input-go was a "tool that takes over
your Mac." With it, downstream agents are "tools that operate apps
in the background while you keep working." That's not a refinement,
that's a different relationship with the user.

`Hold` is small but high signal: it removes a class of failures
that are difficult to reproduce (stuck modifiers manifest only on
certain error paths) and confusing when they happen.

## [0.1.0] - 2026-04-23

Initial release. Pure-Go macOS mouse + keyboard synthesis built on
CGEvent via purego + an embedded ObjC companion dylib. Single binary,
`go install`, no cgo required downstream. Part of the KinKit family
alongside [sckit-go](https://github.com/LocalKinAI/sckit-go) and
[kinrec](https://github.com/LocalKinAI/kinrec).

### Added

#### Mouse API
- `input.Move(ctx, x, y)` — jump cursor to (x, y).
- `input.MoveBy(ctx, dx, dy)` — offset cursor from current position.
- `input.MoveSmooth(ctx, x, y, duration)` — animated move with
  smoothstep easing; ~120 Hz updates bounded to 2-600 steps.
- `input.Click(ctx, button, clicks)` — click at current position.
- `input.ClickAt(ctx, x, y)` — move + single-left-click (most common).
- `input.ClickAtButton(ctx, x, y, button, clicks)` — full control.
- `input.DoubleClick(ctx, x, y)` / `input.RightClick(ctx, x, y)` —
  common convenience wrappers.
- `input.Drag(ctx, fromX, fromY, toX, toY, duration)` — press + eased
  drag + release. On context cancellation, the button is released
  before returning to avoid stuck-pressed state.
- `input.Scroll(ctx, dx, dy)` — wheel event in pixel units.
- `input.ScrollSmooth(ctx, dx, dy, duration)` — distribute scroll
  across ~60 Hz steps with integer carry so totals land exactly on
  (dx, dy).

#### Keyboard API
- `input.KeyDown(ctx, key, mods)` / `input.KeyUp(ctx, key, mods)` —
  held-key primitives.
- `input.Press(ctx, key)` — down + 5 ms gap + up.
- `input.Hotkey(ctx, mods, key)` — modifier-holding combo
  (down mods → down key → up key → up mods, reverse order).
  Supports `ModCommand`, `ModShift`, `ModOption`, `ModControl`,
  `ModFunction` combined via bitwise OR.
- `input.Type(ctx, text)` — UTF-8 input via
  `CGEventKeyboardSetUnicodeString`, bypassing keyboard layout.
  Correctly handles surrogate pairs and emoji grapheme clusters.
- `input.TypeSlow(ctx, text, perCharDelay)` — per-character delay for
  screen demos or rate-limited apps.

#### Keys + modifiers
- `Key` type with named constants for all US-ANSI virtual keycodes:
  letters, digits, Return/Tab/Space/Delete/Escape, arrows, F1-F12,
  Home/End/PageUp/PageDown, symbols, modifier keycodes.
- `input.KeyByName(name)` — case-insensitive lookup with aliases
  (e.g. "enter" → KeyReturn, "esc" → KeyEscape, "pgup" → KeyPageUp).
- `input.ParseModifiers(str)` — parse "cmd+shift" or "cmd,option" into
  Modifier bitmask.
- `Modifier` bits locked to CGEventFlags wire values; regression test
  guards the ABI.

#### Geometry + permission
- `input.CursorPosition()` — (x, y) in global screen coords.
- `input.ScreenSize()` — main display pixel dimensions.
- `input.Trusted()` — probe Accessibility permission without prompting.
- `input.PromptTrust()` — trigger system dialog on first call.
- `input.RequireTrust()` — convenience: returns `ErrNotTrusted` if
  permission is missing, for callers who want hard-fail semantics
  rather than macOS's silent-noop default.

#### Packaging
- `//go:embed` universal dylib (arm64 + x86_64, ~60 KB) —
  downstream users never need `clang` / `CGO_ENABLED` / a C toolchain.
- Auto-extracts to `~/Library/Caches/input-go/<content-hash>/` on
  first call.
- `DylibPath` override for contributors shipping patched dylibs.
- `ResolvedDylibPath()` for diagnostics.

### CLI — `cmd/input`
- `input move X Y [--smooth D]`
- `input click [X Y] [--button left|right|other] [--count N]`
- `input drag FROM_X FROM_Y TO_X TO_Y [--duration D]`
- `input scroll DX DY [--smooth D]`
- `input type TEXT [--delay D]`
- `input press KEY`
- `input hotkey MODS KEY` (e.g. `input hotkey cmd c`)
- `input cursor` / `input screen` — introspection
- `input trust [--prompt]` — permission check
- `input version` — version + Go toolchain + resolved dylib path
- Graceful cancellation on SIGINT / SIGTERM.

### Dylib / ObjC
- 8 exported C-ABI functions:
  `input_ax_trusted` / `input_cursor_position` / `input_screen_size` /
  `input_mouse_event` / `input_scroll` / `input_key` /
  `input_type_unicode` / `input_sleep_ms`.
- Unified `input_mouse_event(x, y, button, kind, clicks)` covering all
  four event kinds (move/down/up/drag) — one symbol per button group
  instead of 12.
- Unicode-path typing uses grapheme-cluster iteration so multi-unichar
  characters (emoji, ZWJ sequences) are one "keystroke" per cluster,
  not per UTF-16 unit.

### Documentation
- `README.md` — install, usage, library API, CLI reference,
  permission model, roadmap.
- `CHANGELOG.md` — this file.

### Tests
- **Unit tests** (no dylib, no permission): keycode enum values
  (ABI stability gate), `KeyByName` + aliases, `ParseModifiers`,
  `expandMods` canonical ordering, `modKeycode` mapping, Modifier
  bitmask values match CGEventFlags, sentinel-error distinctness.
- **Integration tests** (build-tag `integration`, requires
  Accessibility permission): Load, CursorPosition, ScreenSize,
  Move changes cursor position, MoveSmooth lands on target,
  Scroll does not error, RequireTrust returns sentinel.
- `go vet` + `staticcheck` + `golangci-lint` (same 9-linter config as
  sckit-go): **0 warnings**.

### Known limitations

- macOS only (Linux/Windows backends are out of scope).
- No input event *capture* — only synthesis. `input-go/listen` planned for v0.2.
- Unicode typing bypasses keyboard layout (apps watching raw keycodes
  won't see modifier presses).
- CapsLock state is not tracked.
- Tested only on macOS 26.3 arm64 so far; Intel + macOS 14/15 pending CI.
