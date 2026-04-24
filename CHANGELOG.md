# Changelog

All notable changes to input-go are documented here.

Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [SemVer](https://semver.org/spec/v2.0.0.html).

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
