package input

import (
	"errors"
	"testing"
)

func TestVersionSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	// errors.Is should distinguish them; otherwise callers can't do
	// "if errors.Is(err, ErrNotTrusted)" reliably.
	if errors.Is(ErrNotTrusted, ErrInvalidArg) {
		t.Error("ErrNotTrusted and ErrInvalidArg must be distinct")
	}
	if errors.Is(ErrInvalidArg, ErrNotTrusted) {
		t.Error("ErrNotTrusted and ErrInvalidArg must be distinct")
	}
}

func TestMouseButtonConstants(t *testing.T) {
	// These are part of the stable API — lock them down.
	if ButtonLeft != 0 || ButtonRight != 1 || ButtonOther != 2 {
		t.Errorf("MouseButton enum values changed: left=%d right=%d other=%d",
			ButtonLeft, ButtonRight, ButtonOther)
	}
}

func TestModifierConstantsMatchCGEventFlags(t *testing.T) {
	// Values must match the CGEventFlags header; misalignment would
	// silently send wrong modifier combos.
	cases := []struct {
		name string
		got  Modifier
		want Modifier
	}{
		{"Command", ModCommand, 1 << 20},
		{"Shift", ModShift, 1 << 17},
		{"Option", ModOption, 1 << 19},
		{"Control", ModControl, 1 << 18},
		{"Function", ModFunction, 1 << 23},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: %#x want %#x", c.name, c.got, c.want)
		}
	}
}
