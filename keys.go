package input

import "strings"

// Key is a macOS virtual keycode. Values match <Carbon/HIToolbox/Events.h>.
//
// Use the named constants (KeyA, KeySpace, ...) rather than numeric
// values — Apple's keycodes are ANSI-layout specific and not obvious.
type Key int32

// US-ANSI virtual keycodes — stable across macOS versions.
// Source: <HIToolbox/Events.h> kVK_* constants.
const (
	KeyA Key = 0x00
	KeyB Key = 0x0B
	KeyC Key = 0x08
	KeyD Key = 0x02
	KeyE Key = 0x0E
	KeyF Key = 0x03
	KeyG Key = 0x05
	KeyH Key = 0x04
	KeyI Key = 0x22
	KeyJ Key = 0x26
	KeyK Key = 0x28
	KeyL Key = 0x25
	KeyM Key = 0x2E
	KeyN Key = 0x2D
	KeyO Key = 0x1F
	KeyP Key = 0x23
	KeyQ Key = 0x0C
	KeyR Key = 0x0F
	KeyS Key = 0x01
	KeyT Key = 0x11
	KeyU Key = 0x20
	KeyV Key = 0x09
	KeyW Key = 0x0D
	KeyX Key = 0x07
	KeyY Key = 0x10
	KeyZ Key = 0x06

	Key0 Key = 0x1D
	Key1 Key = 0x12
	Key2 Key = 0x13
	Key3 Key = 0x14
	Key4 Key = 0x15
	Key5 Key = 0x17
	Key6 Key = 0x16
	Key7 Key = 0x1A
	Key8 Key = 0x1C
	Key9 Key = 0x19

	KeyReturn        Key = 0x24
	KeyTab           Key = 0x30
	KeySpace         Key = 0x31
	KeyDelete        Key = 0x33 // Backspace on PC keyboards
	KeyEscape        Key = 0x35
	KeyForwardDelete Key = 0x75 // fn+delete / dedicated Del key

	KeyLeftCommand  Key = 0x37
	KeyLeftShift    Key = 0x38
	KeyCapsLock     Key = 0x39
	KeyLeftOption   Key = 0x3A
	KeyLeftControl  Key = 0x3B
	KeyRightShift   Key = 0x3C
	KeyRightOption  Key = 0x3D
	KeyRightControl Key = 0x3E
	KeyFn           Key = 0x3F

	KeyArrowLeft  Key = 0x7B
	KeyArrowRight Key = 0x7C
	KeyArrowDown  Key = 0x7D
	KeyArrowUp    Key = 0x7E

	KeyF1  Key = 0x7A
	KeyF2  Key = 0x78
	KeyF3  Key = 0x63
	KeyF4  Key = 0x76
	KeyF5  Key = 0x60
	KeyF6  Key = 0x61
	KeyF7  Key = 0x62
	KeyF8  Key = 0x64
	KeyF9  Key = 0x65
	KeyF10 Key = 0x6D
	KeyF11 Key = 0x67
	KeyF12 Key = 0x6F

	KeyHome     Key = 0x73
	KeyEnd      Key = 0x77
	KeyPageUp   Key = 0x74
	KeyPageDown Key = 0x79

	KeyMinus        Key = 0x1B
	KeyEqual        Key = 0x18
	KeyLeftBracket  Key = 0x21
	KeyRightBracket Key = 0x1E
	KeyBackslash    Key = 0x2A
	KeySemicolon    Key = 0x29
	KeyQuote        Key = 0x27
	KeyComma        Key = 0x2B
	KeyPeriod       Key = 0x2F
	KeySlash        Key = 0x2C
	KeyGrave        Key = 0x32
)

// KeyByName looks up a Key by its common name (case-insensitive). Used
// by the CLI so `input key --press enter` works. Returns (0, false)
// if the name is unknown.
func KeyByName(name string) (Key, bool) {
	k, ok := keyNameMap[strings.ToLower(name)]
	return k, ok
}

var keyNameMap = map[string]Key{
	// Letters
	"a": KeyA, "b": KeyB, "c": KeyC, "d": KeyD, "e": KeyE, "f": KeyF,
	"g": KeyG, "h": KeyH, "i": KeyI, "j": KeyJ, "k": KeyK, "l": KeyL,
	"m": KeyM, "n": KeyN, "o": KeyO, "p": KeyP, "q": KeyQ, "r": KeyR,
	"s": KeyS, "t": KeyT, "u": KeyU, "v": KeyV, "w": KeyW, "x": KeyX,
	"y": KeyY, "z": KeyZ,

	// Digits
	"0": Key0, "1": Key1, "2": Key2, "3": Key3, "4": Key4,
	"5": Key5, "6": Key6, "7": Key7, "8": Key8, "9": Key9,

	// Named keys
	"return": KeyReturn, "enter": KeyReturn,
	"tab":   KeyTab,
	"space": KeySpace, "spacebar": KeySpace,
	"delete": KeyDelete, "backspace": KeyDelete,
	"forward_delete": KeyForwardDelete, "fwd_delete": KeyForwardDelete,
	"escape": KeyEscape, "esc": KeyEscape,

	// Modifiers
	"cmd": KeyLeftCommand, "command": KeyLeftCommand,
	"shift": KeyLeftShift,
	"opt":   KeyLeftOption, "option": KeyLeftOption, "alt": KeyLeftOption,
	"ctrl": KeyLeftControl, "control": KeyLeftControl,
	"fn":   KeyFn,
	"caps": KeyCapsLock, "capslock": KeyCapsLock,

	// Arrows
	"left": KeyArrowLeft, "right": KeyArrowRight,
	"up": KeyArrowUp, "down": KeyArrowDown,

	// Function keys
	"f1": KeyF1, "f2": KeyF2, "f3": KeyF3, "f4": KeyF4,
	"f5": KeyF5, "f6": KeyF6, "f7": KeyF7, "f8": KeyF8,
	"f9": KeyF9, "f10": KeyF10, "f11": KeyF11, "f12": KeyF12,

	// Navigation
	"home": KeyHome, "end": KeyEnd,
	"pageup": KeyPageUp, "pgup": KeyPageUp,
	"pagedown": KeyPageDown, "pgdn": KeyPageDown,

	// Symbols (US-ANSI)
	"minus": KeyMinus, "-": KeyMinus,
	"equal": KeyEqual, "=": KeyEqual,
	"lbracket": KeyLeftBracket, "[": KeyLeftBracket,
	"rbracket": KeyRightBracket, "]": KeyRightBracket,
	"backslash": KeyBackslash, "\\": KeyBackslash,
	"semicolon": KeySemicolon, ";": KeySemicolon,
	"quote": KeyQuote, "'": KeyQuote,
	"comma": KeyComma, ",": KeyComma,
	"period": KeyPeriod, ".": KeyPeriod,
	"slash": KeySlash, "/": KeySlash,
	"grave": KeyGrave, "`": KeyGrave,
}

// ParseModifiers parses a comma- or '+'-delimited modifier string
// like "cmd+shift" or "cmd,option" into a [Modifier] bitmask. Unknown
// tokens return (0, false). Empty string returns (0, true).
func ParseModifiers(s string) (Modifier, bool) {
	if s == "" {
		return 0, true
	}
	var mods Modifier
	// Accept both '+' and ',' as separators.
	for _, sep := range []string{"+", ","} {
		s = strings.ReplaceAll(s, sep, " ")
	}
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		m, ok := modNameMap[tok]
		if !ok {
			return 0, false
		}
		mods |= m
	}
	return mods, true
}

var modNameMap = map[string]Modifier{
	"cmd": ModCommand, "command": ModCommand, "meta": ModCommand,
	"shift": ModShift,
	"opt":   ModOption, "option": ModOption, "alt": ModOption,
	"ctrl": ModControl, "control": ModControl,
	"fn": ModFunction,
}
