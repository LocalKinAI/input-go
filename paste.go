package input

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PasteText injects a string by writing to the macOS clipboard,
// firing ⌘V via the standard [Hotkey] path, and restoring the
// user's previous clipboard text ~150ms later.
//
// Why a separate function rather than just [Type]: [Type] synthesizes
// a key event per character at full keyboard rate (~100 char/s).
// IME front-ends (Pinyin / Wubi / Kotoeri) frequently can't keep up
// with that rate — characters get dropped or the IME's composition
// state desyncs. PasteText sidesteps the IME entirely by routing
// through the system pasteboard.
//
// Restore semantics: PasteText always saves the previous clipboard
// before writing + restores it after the keystroke. Non-text
// pasteboard content (image / file / RTF) does NOT survive the
// round-trip — by macOS pasteboard design — but plain text (the
// 95% case) does. If you don't want the restore, set the clipboard
// yourself with [WriteClipboard] after PasteText returns.
//
// Routing: [PostOption]s (e.g. [WithPID]) apply to the ⌘V keystroke,
// so paste can target a background app without stealing focus from
// the user's foreground window. The pbcopy/pbpaste shell-outs are
// system-wide either way — there's only one macOS pasteboard.
//
//	if err := input.PasteText(ctx, "你好世界"); err != nil { ... }
//	input.PasteText(ctx, longChineseText, input.WithPID(pid))
//
// Errors:
//
//   - pbcopy / pbpaste failures propagate as "pbcopy: …" / "pbpaste: …".
//     Rare unless running in a sandbox without /usr/bin.
//   - The ⌘V keystroke uses the same error chain as [Hotkey].
func PasteText(ctx context.Context, text string, opts ...PostOption) error {
	// 1. Save current clipboard (best-effort).
	prev, _ := pbpaste(ctx)

	// 2. Write the new text.
	if err := pbcopy(ctx, text); err != nil {
		return fmt.Errorf("input.PasteText: pbcopy: %w", err)
	}

	// 3. Fire ⌘V via the existing Hotkey path so target_pid + delay
	// opts route consistently with the rest of input-go.
	mods, _ := ParseModifiers("cmd")
	v, ok := KeyByName("v")
	if !ok {
		return fmt.Errorf("input.PasteText: cannot resolve key 'v'")
	}
	if err := Hotkey(ctx, mods, v, opts...); err != nil {
		return fmt.Errorf("input.PasteText: ⌘V: %w", err)
	}

	// 4. Settle so the target app reads the clipboard before we
	// overwrite it. 150ms is generous for AppKit / WebKit; Electron
	// can take 200-300ms on cold starts but they almost always
	// already read during the press.
	time.Sleep(150 * time.Millisecond)

	// 5. Restore previous clipboard. Best-effort — if it fails, the
	// text was already pasted successfully so we don't bubble the
	// error up.
	if prev != "" {
		_ = pbcopy(ctx, prev)
	}
	return nil
}

// ReadClipboard returns the macOS clipboard's current text content
// via pbpaste. Useful as a building block for code that needs to
// inspect / round-trip the pasteboard without going through the
// whole PasteText flow.
func ReadClipboard(ctx context.Context) (string, error) {
	return pbpaste(ctx)
}

// WriteClipboard writes text to the macOS clipboard via pbcopy.
// Companion to [ReadClipboard]. Use this if you want to populate
// the clipboard without immediately firing ⌘V.
func WriteClipboard(ctx context.Context, text string) error {
	return pbcopy(ctx, text)
}

// pbcopy / pbpaste shell out to macOS pasteboard CLI tools. They
// pipe via stdin/stdout so binary-safe up to the user's locale —
// good enough for arbitrary UTF-8 text. For richer typed
// pasteboard content (RTF, images, files) you'd need NSPasteboard
// via cgo / a future kinpaste-go kit; not needed for the "fast
// text" charter of these helpers.
func pbcopy(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func pbpaste(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "pbpaste").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
