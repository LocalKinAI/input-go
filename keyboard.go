package input

import (
	"context"
	"fmt"
	"time"
	"unsafe"
)

// Modifier is a bitmask of keyboard modifier flags. Combine with bitwise
// OR: `input.ModCommand | input.ModShift`. Values match CGEventFlags.
type Modifier uint64

const (
	// ModCommand is the Command (⌘) modifier.
	ModCommand Modifier = 1 << 20 // kCGEventFlagMaskCommand
	// ModShift is the Shift (⇧) modifier.
	ModShift Modifier = 1 << 17 // kCGEventFlagMaskShift
	// ModOption is the Option/Alt (⌥) modifier.
	ModOption Modifier = 1 << 19 // kCGEventFlagMaskAlternate
	// ModControl is the Control (⌃) modifier.
	ModControl Modifier = 1 << 18 // kCGEventFlagMaskControl
	// ModFunction is the Function (fn) modifier.
	ModFunction Modifier = 1 << 23 // kCGEventFlagMaskSecondaryFn
)

// KeyDown posts a key-down event for `key`. Optionally carries modifier
// flags — pass 0 for no modifiers.
func KeyDown(ctx context.Context, key Key, mods Modifier) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if rc := keyFn(int32(key), 1, uint64(mods)); rc != 0 {
		return fmt.Errorf("input: key down rc=%d", rc)
	}
	return nil
}

// KeyUp posts a key-up event for `key`.
func KeyUp(ctx context.Context, key Key, mods Modifier) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if rc := keyFn(int32(key), 0, uint64(mods)); rc != 0 {
		return fmt.Errorf("input: key up rc=%d", rc)
	}
	return nil
}

// Press taps a key: down then up. This is the common case for any key
// that doesn't need to be held.
func Press(ctx context.Context, key Key) error {
	if err := KeyDown(ctx, key, 0); err != nil {
		return err
	}
	// Small gap so apps register distinct down/up rather than coalescing.
	time.Sleep(5 * time.Millisecond)
	return KeyUp(ctx, key, 0)
}

// Hotkey presses `key` with `mods` held: down-modifiers → down-key →
// up-key → up-modifiers. This is how you send Cmd+C, Cmd+Shift+T, etc.
//
//	input.Hotkey(ctx, input.ModCommand, input.KeyC)                 // copy
//	input.Hotkey(ctx, input.ModCommand|input.ModShift, input.KeyT)  // reopen tab
func Hotkey(ctx context.Context, mods Modifier, key Key) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Press each modifier. Modifier flags layer — we track the
	// accumulated flag set and pass it with each event.
	var accum Modifier
	for _, m := range expandMods(mods) {
		accum |= m
		if rc := keyFn(int32(modKeycode(m)), 1, uint64(accum)); rc != 0 {
			return fmt.Errorf("input: mod down rc=%d", rc)
		}
	}

	// Press the key, with modifiers held.
	if rc := keyFn(int32(key), 1, uint64(accum)); rc != 0 {
		return fmt.Errorf("input: hotkey down rc=%d", rc)
	}
	time.Sleep(5 * time.Millisecond)
	if rc := keyFn(int32(key), 0, uint64(accum)); rc != 0 {
		return fmt.Errorf("input: hotkey up rc=%d", rc)
	}

	// Release modifiers in reverse order.
	modList := expandMods(mods)
	for i := len(modList) - 1; i >= 0; i-- {
		m := modList[i]
		accum &^= m
		if rc := keyFn(int32(modKeycode(m)), 0, uint64(accum)); rc != 0 {
			return fmt.Errorf("input: mod up rc=%d", rc)
		}
	}
	return nil
}

// expandMods returns the ordered list of individual modifier bits set.
func expandMods(m Modifier) []Modifier {
	var out []Modifier
	// Canonical order: Cmd, Option, Control, Shift, Fn.
	for _, bit := range []Modifier{ModCommand, ModOption, ModControl, ModShift, ModFunction} {
		if m&bit != 0 {
			out = append(out, bit)
		}
	}
	return out
}

// modKeycode returns the left-side virtual keycode for a modifier.
// We pick left-side variants for determinism; apps can't distinguish.
func modKeycode(m Modifier) Key {
	switch m {
	case ModCommand:
		return KeyLeftCommand
	case ModOption:
		return KeyLeftOption
	case ModControl:
		return KeyLeftControl
	case ModShift:
		return KeyLeftShift
	case ModFunction:
		return KeyFn
	}
	return Key(0)
}

// ─── Typing text ─────────────────────────────────────────────

// Type synthesizes keyboard events that produce `text` in the currently
// focused text field. Works for arbitrary UTF-8 including emoji and
// CJK, because it uses CGEventKeyboardSetUnicodeString rather than
// translating through the current keyboard layout.
//
// Note: because this bypasses the keyboard layout, typing "A" is a
// literal A glyph, not "shift+a". Apps that listen for raw keycodes
// (games, key remappers) won't see a modifier press.
func Type(ctx context.Context, text string) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	b := []byte(text)
	if rc := typeUnicodeFn(unsafe.Pointer(&b[0]), int32(len(b))); rc != 0 {
		return fmt.Errorf("input: type rc=%d", rc)
	}
	return nil
}

// TypeSlow is like [Type] but inserts `perCharDelay` between characters.
// Useful for demos where "instant paste" looks artificial, or for apps
// that throttle rapid-fire input.
func TypeSlow(ctx context.Context, text string, perCharDelay time.Duration) error {
	if perCharDelay <= 0 {
		return Type(ctx, text)
	}
	for _, r := range text {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := Type(ctx, string(r)); err != nil {
			return err
		}
		time.Sleep(perCharDelay)
	}
	return nil
}
