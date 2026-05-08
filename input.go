// Package input is a pure-Go macOS mouse and keyboard synthesis library.
//
// input uses CGEvent (Quartz Event Services) to post mouse and keyboard
// events system-wide. It's the foundation for automation, UI testing,
// screen-demo tooling, and remote-control agents — anything that needs
// to drive the cursor or type text.
//
// # Quick start
//
//	ctx := context.Background()
//	if err := input.Load(); err != nil { log.Fatal(err) }
//
//	// Move cursor and click
//	input.ClickAt(ctx, 400, 300)
//
//	// Type text
//	input.Type(ctx, "hello, world")
//
//	// Cmd+C hotkey
//	input.Hotkey(ctx, input.ModCommand, input.KeyC)
//
// # Permissions
//
// macOS requires the invoking binary to be listed in System Settings →
// Privacy & Security → Accessibility for synthesized events to take
// effect. Without permission, calls return nil (no error) but the
// events have no visible effect — this matches macOS's silent-failure
// model. Use [Trusted] to check, and [PromptTrust] to trigger the
// system dialog.
//
// # Dylib placement
//
// input-go ships a universal (arm64+x86_64) companion dylib via
// //go:embed. On the first call into the package, the embedded bytes
// are extracted to ~/Library/Caches/input-go/<hash>/libinput_sync.dylib
// and Dlopened. Set [DylibPath] to a non-empty value before the first
// call if you ship a custom-built or patched dylib.
//
// [CGEvent]: https://developer.apple.com/documentation/coregraphics/quartz_event_services
package input

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/LocalKinAI/input-go/internal/dylib"
	"github.com/ebitengine/purego"
)

// Version is the semantic-version tag of this package.
// Kept in sync with git tags; updated per release.
const Version = "0.3.0"

// DylibPath is an optional override for the location of libinput_sync.dylib.
// Default (empty): extract the embedded copy to the user cache directory.
// Set to a non-empty path BEFORE the first call into this package if
// shipping a custom-built dylib.
var DylibPath = ""

// ─── Dylib handle (unexported) ───────────────────────────────

var (
	loadOnce sync.Once
	loadErr  error

	axTrustedFn   func(int32) int32
	cursorPosFn   func(unsafe.Pointer, unsafe.Pointer) int32
	screenSizeFn  func(unsafe.Pointer, unsafe.Pointer) int32
	mouseEventFn  func(int32, float64, float64, int32, int32, int32) int32
	scrollFn      func(int32, int32, int32) int32
	keyFn         func(int32, int32, int32, uint64) int32
	typeUnicodeFn func(int32, unsafe.Pointer, int32) int32
	sleepMsFn     func(int32) int32
)

// Load explicitly loads the companion dylib. It's idempotent: subsequent
// calls return the same cached error (or nil).
//
// Resolution order:
//  1. If DylibPath is non-empty, use it (user override).
//  2. Otherwise, extract the embedded universal dylib to the cache
//     directory and Dlopen from there.
//
// Load is called automatically by every public function; the exported
// form exists so applications can fail fast at startup.
func Load() error {
	loadOnce.Do(func() {
		if runtime.GOOS != "darwin" {
			loadErr = fmt.Errorf("input: macOS-only (runtime.GOOS=%s)", runtime.GOOS)
			return
		}
		path, err := resolveDylib()
		if err != nil {
			loadErr = err
			return
		}
		resolvedPathMu.Lock()
		resolvedPath = path
		resolvedPathMu.Unlock()

		h, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("input: dlopen %q: %w", path, err)
			return
		}
		purego.RegisterLibFunc(&axTrustedFn, h, "input_ax_trusted")
		purego.RegisterLibFunc(&cursorPosFn, h, "input_cursor_position")
		purego.RegisterLibFunc(&screenSizeFn, h, "input_screen_size")
		purego.RegisterLibFunc(&mouseEventFn, h, "input_mouse_event")
		purego.RegisterLibFunc(&scrollFn, h, "input_scroll")
		purego.RegisterLibFunc(&keyFn, h, "input_key")
		purego.RegisterLibFunc(&typeUnicodeFn, h, "input_type_unicode")
		purego.RegisterLibFunc(&sleepMsFn, h, "input_sleep_ms")
	})
	return loadErr
}

var (
	resolvedPath   string
	resolvedPathMu sync.RWMutex
)

// ResolvedDylibPath returns the filesystem path that Load used (or would
// use) to Dlopen the dylib. Intended for diagnostics.
func ResolvedDylibPath() string {
	resolvedPathMu.RLock()
	defer resolvedPathMu.RUnlock()
	return resolvedPath
}

func resolveDylib() (string, error) {
	if DylibPath != "" {
		if _, err := os.Stat(DylibPath); err != nil {
			return "", fmt.Errorf("input: DylibPath override %q: %w", DylibPath, err)
		}
		return DylibPath, nil
	}
	return extractEmbedded()
}

func extractEmbedded() (string, error) {
	if len(dylib.Bytes) == 0 {
		return "", errors.New("input: embedded dylib is empty (build issue — make dylib)")
	}
	h := sha256.Sum256(dylib.Bytes)
	hashPrefix := hex.EncodeToString(h[:8])

	baseCache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("input: locate cache dir: %w", err)
	}
	cacheDir := filepath.Join(baseCache, "input-go", hashPrefix)
	target := filepath.Join(cacheDir, "libinput_sync.dylib")

	if existing, err := os.ReadFile(target); err == nil && len(existing) == len(dylib.Bytes) {
		return target, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("input: mkdir %s: %w", cacheDir, err)
	}
	tmp, err := os.CreateTemp(cacheDir, "libinput_sync-*.dylib.tmp")
	if err != nil {
		return "", fmt.Errorf("input: create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(dylib.Bytes); err != nil {
		cleanup()
		return "", fmt.Errorf("input: write dylib: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		cleanup()
		return "", fmt.Errorf("input: chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("input: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("input: rename: %w", err)
	}
	return target, nil
}

// ─── Posting options ─────────────────────────────────────────

// PostOption tunes how an event is posted to the OS. Pass via the
// variadic `opts` tail of any event-posting function.
type PostOption func(*postConfig)

type postConfig struct {
	// pid: 0 = post system-wide via kCGHIDEventTap (the focused app
	// receives the event and may come to front);
	// > 0 = route directly to the given process via CGEventPostToPid
	// (no focus steal).
	pid int32
}

func resolveOpts(opts []PostOption) postConfig {
	var c postConfig
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return c
}

// WithPID routes events directly to the given process via
// CGEventPostToPid, avoiding focus steal — your foreground app stays
// foreground while input-go drives a background app. pid=0 (the
// default when the option is omitted) keeps the legacy system-wide
// HID event tap behavior, which moves focus to the targeted control's
// owning app.
//
// Verified working on Lark / VSCode / Chrome and other Electron + Web
// View hosts. Some Apple sandboxed apps (newer Mail / Messages) may
// ignore PID-targeted events — fall back to the default if you see
// no effect.
//
//	input.Click(ctx, 400, 300, input.WithPID(int32(targetPID)))
//	input.Type(ctx, "hello", input.WithPID(int32(targetPID)))
//	input.Hotkey(ctx, input.ModCommand, input.KeyS, input.WithPID(int32(targetPID)))
func WithPID(pid int32) PostOption {
	return func(c *postConfig) { c.pid = pid }
}

// ─── Sentinel errors ─────────────────────────────────────────

// ErrNotTrusted is returned when the current process lacks Accessibility
// permission AND the caller explicitly requested a trust check (see
// [RequireTrust]). Regular input calls DO NOT return this error — they
// silently no-op, matching macOS behavior.
var ErrNotTrusted = errors.New("input: accessibility permission not granted")

// ErrInvalidArg is returned for malformed inputs (e.g. an empty key, a
// negative click count).
var ErrInvalidArg = errors.New("input: invalid argument")

// ─── Trust helpers ───────────────────────────────────────────

// Trusted returns true if the current process has Accessibility
// permission granted. Does not prompt — use [PromptTrust] for that.
func Trusted() bool {
	if err := Load(); err != nil {
		return false
	}
	return axTrustedFn(0) == 1
}

// PromptTrust triggers the system "<app> wants access to control your
// computer" dialog if permission has not yet been granted. Returns
// current trust state (true if granted, false if the user hasn't
// responded yet or denied).
//
// This is a one-shot prompt — subsequent calls return the current
// state without showing the dialog again (until the user resets TCC).
func PromptTrust() bool {
	if err := Load(); err != nil {
		return false
	}
	return axTrustedFn(1) == 1
}

// RequireTrust returns nil if Accessibility permission is granted, or
// [ErrNotTrusted] otherwise. Convenience for callers that want a hard
// fail rather than silent no-ops.
func RequireTrust() error {
	if !Trusted() {
		return ErrNotTrusted
	}
	return nil
}

// ─── Screen + cursor geometry ────────────────────────────────

// CursorPosition returns the current cursor location in global screen
// coordinates (origin at top-left).
func CursorPosition() (x, y float64, err error) {
	if err := Load(); err != nil {
		return 0, 0, err
	}
	var xv, yv float64
	if rc := cursorPosFn(unsafe.Pointer(&xv), unsafe.Pointer(&yv)); rc != 0 {
		return 0, 0, fmt.Errorf("input: cursor_position rc=%d", rc)
	}
	return xv, yv, nil
}

// ScreenSize returns the main display's pixel dimensions.
func ScreenSize() (w, h float64, err error) {
	if err := Load(); err != nil {
		return 0, 0, err
	}
	var wv, hv float64
	if rc := screenSizeFn(unsafe.Pointer(&wv), unsafe.Pointer(&hv)); rc != 0 {
		return 0, 0, fmt.Errorf("input: screen_size rc=%d", rc)
	}
	return wv, hv, nil
}
