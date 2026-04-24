package input

import "testing"

func TestKeyByName(t *testing.T) {
	cases := []struct {
		name    string
		want    Key
		present bool
	}{
		{"a", KeyA, true},
		{"Z", KeyZ, true},
		{"0", Key0, true},
		{"Return", KeyReturn, true},
		{"enter", KeyReturn, true},
		{"space", KeySpace, true},
		{"esc", KeyEscape, true},
		{"escape", KeyEscape, true},
		{"cmd", KeyLeftCommand, true},
		{"Command", KeyLeftCommand, true},
		{"left", KeyArrowLeft, true},
		{"up", KeyArrowUp, true},
		{"f5", KeyF5, true},
		{"pageup", KeyPageUp, true},
		{"pgup", KeyPageUp, true},
		{"[", KeyLeftBracket, true},
		{",", KeyComma, true},
		{"nonexistent", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := KeyByName(c.name)
		if ok != c.present {
			t.Errorf("KeyByName(%q): ok=%v want %v", c.name, ok, c.present)
			continue
		}
		if ok && got != c.want {
			t.Errorf("KeyByName(%q) = %#x, want %#x", c.name, got, c.want)
		}
	}
}

func TestParseModifiers(t *testing.T) {
	cases := []struct {
		in     string
		want   Modifier
		wantOK bool
	}{
		{"", 0, true},
		{"cmd", ModCommand, true},
		{"command", ModCommand, true},
		{"cmd+shift", ModCommand | ModShift, true},
		{"cmd,shift", ModCommand | ModShift, true},
		{"cmd+shift+option", ModCommand | ModShift | ModOption, true},
		{"CMD+SHIFT", ModCommand | ModShift, true},
		{"ctrl+alt", ModControl | ModOption, true},
		{"bogus", 0, false},
		{"cmd+bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseModifiers(c.in)
		if ok != c.wantOK {
			t.Errorf("ParseModifiers(%q): ok=%v want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && got != c.want {
			t.Errorf("ParseModifiers(%q) = %#x, want %#x", c.in, got, c.want)
		}
	}
}

func TestExpandMods(t *testing.T) {
	got := expandMods(ModCommand | ModShift | ModOption)
	if len(got) != 3 {
		t.Fatalf("expected 3 modifiers, got %d: %v", len(got), got)
	}
	// Expect canonical order: Cmd, Option, Control, Shift, Fn.
	if got[0] != ModCommand || got[1] != ModOption || got[2] != ModShift {
		t.Errorf("expandMods returned non-canonical order: %v", got)
	}
}

func TestModKeycode(t *testing.T) {
	cases := map[Modifier]Key{
		ModCommand:  KeyLeftCommand,
		ModShift:    KeyLeftShift,
		ModOption:   KeyLeftOption,
		ModControl:  KeyLeftControl,
		ModFunction: KeyFn,
	}
	for m, want := range cases {
		if got := modKeycode(m); got != want {
			t.Errorf("modKeycode(%#x) = %#x, want %#x", m, got, want)
		}
	}
}
