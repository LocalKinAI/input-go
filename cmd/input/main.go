// Command input drives the mouse and keyboard from the command line.
//
// Usage:
//
//	input move X Y [--smooth D]
//	input click [X Y] [--button left|right|other] [--count N]
//	input drag FROM_X FROM_Y TO_X TO_Y [--duration D]
//	input scroll DX DY [--smooth D]
//	input type TEXT [--delay D]
//	input press KEY
//	input hotkey MODS KEY        # e.g. input hotkey cmd c
//	input cursor                 # print current cursor position
//	input screen                 # print screen size
//	input trust [--prompt]       # check accessibility permission
//	input version
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/LocalKinAI/input-go"
)

const usage = `input — mouse + keyboard synthesizer (input-go)

COMMANDS
  move X Y [--smooth D]
      Move the cursor to (X, Y). --smooth D animates over duration D.

  click [X Y] [--button left|right|other] [--count N]
      Click at (X, Y) or at the current cursor position.

  drag FROM_X FROM_Y TO_X TO_Y [--duration D]
      Press left button at FROM, drag to TO, release.

  scroll DX DY [--smooth D]
      Scroll by (DX, DY) pixels. --smooth D spreads the scroll over D.

  type TEXT [--delay D]
      Type TEXT into the focused field. --delay D inserts D between chars.

  press KEY
      Tap a single key (e.g. "enter", "escape", "f5", "left").

  hotkey MODS KEY
      Press MODS+KEY combo (e.g. "cmd c", "cmd+shift t").

  cursor
      Print "X Y" — current cursor position.

  screen
      Print "W H" — main display pixel dimensions.

  trust [--prompt]
      Print 1 if Accessibility is granted, 0 otherwise. --prompt shows
      the system dialog on first run.

  version
      Print version info.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "move":
		cmdMove(ctx, args)
	case "click":
		cmdClick(ctx, args)
	case "drag":
		cmdDrag(ctx, args)
	case "scroll":
		cmdScroll(ctx, args)
	case "type":
		cmdType(ctx, args)
	case "press":
		cmdPress(ctx, args)
	case "hotkey":
		cmdHotkey(ctx, args)
	case "cursor":
		cmdCursor()
	case "screen":
		cmdScreen()
	case "trust":
		cmdTrust(args)
	case "version":
		fmt.Printf("input-go %s (go %s, %s/%s)\n", input.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		_ = input.Load()
		fmt.Printf("dylib: %s\n", input.ResolvedDylibPath())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "input: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

func cmdMove(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("move", flag.ExitOnError)
	smooth := fs.Duration("smooth", 0, "animate over this duration")
	_ = fs.Parse(args)
	if fs.NArg() != 2 {
		fatalf("move: expected X Y\n")
	}
	x := mustFloat(fs.Arg(0), "X")
	y := mustFloat(fs.Arg(1), "Y")
	var err error
	if *smooth > 0 {
		err = input.MoveSmooth(ctx, x, y, *smooth)
	} else {
		err = input.Move(ctx, x, y)
	}
	check(err)
}

func cmdClick(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("click", flag.ExitOnError)
	btn := fs.String("button", "left", "left|right|other")
	count := fs.Int("count", 1, "click count (1=single, 2=double, ...)")
	_ = fs.Parse(args)

	button, err := parseButton(*btn)
	check(err)

	switch fs.NArg() {
	case 0:
		check(input.Click(ctx, button, *count))
	case 2:
		x := mustFloat(fs.Arg(0), "X")
		y := mustFloat(fs.Arg(1), "Y")
		check(input.ClickAtButton(ctx, x, y, button, *count))
	default:
		fatalf("click: expected 0 or 2 positional args\n")
	}
}

func cmdDrag(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("drag", flag.ExitOnError)
	dur := fs.Duration("duration", 250*time.Millisecond, "drag animation duration")
	_ = fs.Parse(args)
	if fs.NArg() != 4 {
		fatalf("drag: expected FROM_X FROM_Y TO_X TO_Y\n")
	}
	fx := mustFloat(fs.Arg(0), "FROM_X")
	fy := mustFloat(fs.Arg(1), "FROM_Y")
	tx := mustFloat(fs.Arg(2), "TO_X")
	ty := mustFloat(fs.Arg(3), "TO_Y")
	check(input.Drag(ctx, fx, fy, tx, ty, *dur))
}

func cmdScroll(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("scroll", flag.ExitOnError)
	smooth := fs.Duration("smooth", 0, "spread scroll over this duration")
	_ = fs.Parse(args)
	if fs.NArg() != 2 {
		fatalf("scroll: expected DX DY\n")
	}
	dx := mustInt(fs.Arg(0), "DX")
	dy := mustInt(fs.Arg(1), "DY")
	var err error
	if *smooth > 0 {
		err = input.ScrollSmooth(ctx, dx, dy, *smooth)
	} else {
		err = input.Scroll(ctx, dx, dy)
	}
	check(err)
}

func cmdType(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("type", flag.ExitOnError)
	delay := fs.Duration("delay", 0, "delay between chars")
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatalf("type: expected TEXT\n")
	}
	// Join remaining args with spaces so multi-word strings don't need
	// shell quoting (though still recommended for special chars).
	text := fs.Arg(0)
	for i := 1; i < fs.NArg(); i++ {
		text += " " + fs.Arg(i)
	}
	var err error
	if *delay > 0 {
		err = input.TypeSlow(ctx, text, *delay)
	} else {
		err = input.Type(ctx, text)
	}
	check(err)
}

func cmdPress(ctx context.Context, args []string) {
	if len(args) != 1 {
		fatalf("press: expected KEY\n")
	}
	k, ok := input.KeyByName(args[0])
	if !ok {
		fatalf("press: unknown key %q\n", args[0])
	}
	check(input.Press(ctx, k))
}

func cmdHotkey(ctx context.Context, args []string) {
	if len(args) != 2 {
		fatalf("hotkey: expected MODS KEY (e.g. cmd c)\n")
	}
	mods, ok := input.ParseModifiers(args[0])
	if !ok {
		fatalf("hotkey: unknown modifier %q\n", args[0])
	}
	k, ok := input.KeyByName(args[1])
	if !ok {
		fatalf("hotkey: unknown key %q\n", args[1])
	}
	check(input.Hotkey(ctx, mods, k))
}

func cmdCursor() {
	x, y, err := input.CursorPosition()
	check(err)
	fmt.Printf("%.0f %.0f\n", x, y)
}

func cmdScreen() {
	w, h, err := input.ScreenSize()
	check(err)
	fmt.Printf("%.0f %.0f\n", w, h)
}

func cmdTrust(args []string) {
	fs := flag.NewFlagSet("trust", flag.ExitOnError)
	prompt := fs.Bool("prompt", false, "show system dialog on first check")
	_ = fs.Parse(args)

	var ok bool
	if *prompt {
		ok = input.PromptTrust()
	} else {
		ok = input.Trusted()
	}
	if ok {
		fmt.Println("1")
	} else {
		fmt.Println("0")
		os.Exit(1)
	}
}

// ─── helpers ─────────────────────────────────────────────────

func parseButton(s string) (input.MouseButton, error) {
	switch s {
	case "left":
		return input.ButtonLeft, nil
	case "right":
		return input.ButtonRight, nil
	case "other", "middle":
		return input.ButtonOther, nil
	default:
		return 0, fmt.Errorf("unknown button %q", s)
	}
}

func mustFloat(s, name string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fatalf("invalid %s %q: %v\n", name, s, err)
	}
	return v
}

func mustInt(s, name string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		fatalf("invalid %s %q: %v\n", name, s, err)
	}
	return v
}

func check(err error) {
	if err != nil {
		fatalf("error: %v\n", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
