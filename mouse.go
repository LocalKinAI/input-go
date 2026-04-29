package input

import (
	"context"
	"fmt"
	"math"
	"time"
)

// MouseButton enumerates the three mouse buttons that CGEvent understands.
type MouseButton int32

const (
	ButtonLeft  MouseButton = 0
	ButtonRight MouseButton = 1
	ButtonOther MouseButton = 2
)

// Internal CGEvent "kind" values matching objc/input_events.m.
const (
	mouseKindMove = 0
	mouseKindDown = 1
	mouseKindUp   = 2
	mouseKindDrag = 3
)

// Move jumps the cursor instantly to (x, y) in global screen coordinates.
// (0, 0) is the top-left of the main display. Pass [WithPID] to route the
// event to a specific app without focus steal.
func Move(ctx context.Context, x, y float64, opts ...PostOption) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg := resolveOpts(opts)
	if rc := mouseEventFn(cfg.pid, x, y, int32(ButtonLeft), mouseKindMove, 0); rc != 0 {
		return fmt.Errorf("input: move rc=%d", rc)
	}
	return nil
}

// MoveBy offsets the cursor by (dx, dy) from its current position.
func MoveBy(ctx context.Context, dx, dy float64, opts ...PostOption) error {
	x, y, err := CursorPosition()
	if err != nil {
		return err
	}
	return Move(ctx, x+dx, y+dy, opts...)
}

// MoveSmooth moves the cursor from its current position to (x, y) by
// linearly interpolating through intermediate points. `duration`
// controls total animation length; shorter values feel snappier. A
// `duration` of 0 degenerates to [Move].
//
// This is the animation primitive used for "screen demo" recordings
// where a teleporting cursor looks jarring.
func MoveSmooth(ctx context.Context, x, y float64, duration time.Duration, opts ...PostOption) error {
	if duration <= 0 {
		return Move(ctx, x, y, opts...)
	}
	if err := Load(); err != nil {
		return err
	}
	cfg := resolveOpts(opts)

	sx, sy, err := CursorPosition()
	if err != nil {
		return err
	}

	// Target ~120 Hz update rate for smooth-looking motion. Bound the
	// step count so absurd durations don't blow up the schedule.
	steps := int(duration / (time.Second / 120))
	if steps < 2 {
		steps = 2
	}
	if steps > 600 {
		steps = 600
	}
	stepDur := duration / time.Duration(steps)

	for i := 1; i <= steps; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		t := float64(i) / float64(steps)
		// Smoothstep easing: t * t * (3 - 2t) — feels natural.
		eased := t * t * (3 - 2*t)
		px := sx + (x-sx)*eased
		py := sy + (y-sy)*eased
		if rc := mouseEventFn(cfg.pid, px, py, int32(ButtonLeft), mouseKindMove, 0); rc != 0 {
			return fmt.Errorf("input: move rc=%d", rc)
		}
		time.Sleep(stepDur)
	}
	return nil
}

// ─── Clicks ──────────────────────────────────────────────────

// Click presses and releases `button` at the current cursor position.
// `clicks` is the click count (1 = single, 2 = double, 3 = triple).
// A zero or negative `clicks` is treated as 1.
func Click(ctx context.Context, button MouseButton, clicks int, opts ...PostOption) error {
	if clicks < 1 {
		clicks = 1
	}
	x, y, err := CursorPosition()
	if err != nil {
		return err
	}
	return clickAt(ctx, x, y, button, clicks, resolveOpts(opts).pid)
}

// ClickAt moves the cursor to (x, y) and single-left-clicks. This is
// the most common convenience — 90% of automation needs it.
func ClickAt(ctx context.Context, x, y float64, opts ...PostOption) error {
	return clickAt(ctx, x, y, ButtonLeft, 1, resolveOpts(opts).pid)
}

// ClickAtButton is like [ClickAt] but with a configurable button and
// click count.
func ClickAtButton(ctx context.Context, x, y float64, button MouseButton, clicks int, opts ...PostOption) error {
	if clicks < 1 {
		clicks = 1
	}
	return clickAt(ctx, x, y, button, clicks, resolveOpts(opts).pid)
}

// DoubleClick is shorthand for ClickAtButton(..., ButtonLeft, 2).
func DoubleClick(ctx context.Context, x, y float64, opts ...PostOption) error {
	return clickAt(ctx, x, y, ButtonLeft, 2, resolveOpts(opts).pid)
}

// RightClick is shorthand for ClickAtButton(..., ButtonRight, 1).
func RightClick(ctx context.Context, x, y float64, opts ...PostOption) error {
	return clickAt(ctx, x, y, ButtonRight, 1, resolveOpts(opts).pid)
}

func clickAt(ctx context.Context, x, y float64, button MouseButton, clicks int, pid int32) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Move, then down, then up. All events at (x, y) — macOS figures out
	// the click count from the kCGMouseEventClickState field we set.
	if rc := mouseEventFn(pid, x, y, int32(button), mouseKindMove, 0); rc != 0 {
		return fmt.Errorf("input: click move rc=%d", rc)
	}
	if rc := mouseEventFn(pid, x, y, int32(button), mouseKindDown, int32(clicks)); rc != 0 {
		return fmt.Errorf("input: click down rc=%d", rc)
	}
	if rc := mouseEventFn(pid, x, y, int32(button), mouseKindUp, int32(clicks)); rc != 0 {
		return fmt.Errorf("input: click up rc=%d", rc)
	}
	return nil
}

// ─── Drag ────────────────────────────────────────────────────

// Drag performs a "mouse-down → move → mouse-up" sequence: press the
// left button at `from`, interpolate the cursor toward `to` over
// `duration`, release. Used for drag-select, drag-to-reorder, slider
// control, etc.
func Drag(ctx context.Context, fromX, fromY, toX, toY float64, duration time.Duration, opts ...PostOption) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg := resolveOpts(opts)

	// Jump to start, press down.
	if rc := mouseEventFn(cfg.pid, fromX, fromY, int32(ButtonLeft), mouseKindMove, 0); rc != 0 {
		return fmt.Errorf("input: drag move rc=%d", rc)
	}
	if rc := mouseEventFn(cfg.pid, fromX, fromY, int32(ButtonLeft), mouseKindDown, 1); rc != 0 {
		return fmt.Errorf("input: drag down rc=%d", rc)
	}

	if duration < 0 {
		duration = 0
	}
	steps := int(duration / (time.Second / 120))
	if steps < 2 {
		steps = 2
	}
	if steps > 600 {
		steps = 600
	}
	var stepDur time.Duration
	if duration > 0 {
		stepDur = duration / time.Duration(steps)
	}

	for i := 1; i <= steps; i++ {
		if err := ctx.Err(); err != nil {
			// Best effort: release the button so we don't leave the
			// system in a stuck-pressed state.
			_ = mouseEventFn(cfg.pid, toX, toY, int32(ButtonLeft), mouseKindUp, 1)
			return err
		}
		t := float64(i) / float64(steps)
		eased := t * t * (3 - 2*t)
		px := fromX + (toX-fromX)*eased
		py := fromY + (toY-fromY)*eased
		if rc := mouseEventFn(cfg.pid, px, py, int32(ButtonLeft), mouseKindDrag, 0); rc != 0 {
			_ = mouseEventFn(cfg.pid, toX, toY, int32(ButtonLeft), mouseKindUp, 1)
			return fmt.Errorf("input: drag step rc=%d", rc)
		}
		if stepDur > 0 {
			time.Sleep(stepDur)
		}
	}

	if rc := mouseEventFn(cfg.pid, toX, toY, int32(ButtonLeft), mouseKindUp, 1); rc != 0 {
		return fmt.Errorf("input: drag up rc=%d", rc)
	}
	return nil
}

// ─── Scroll ──────────────────────────────────────────────────

// Scroll posts a scroll-wheel event. `dy` is vertical (positive scrolls
// content up — same as flicking your finger upward on a trackpad);
// `dx` is horizontal (positive scrolls right). Units are pixels.
func Scroll(ctx context.Context, dx, dy int, opts ...PostOption) error {
	if err := Load(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg := resolveOpts(opts)
	if rc := scrollFn(cfg.pid, int32(dx), int32(dy)); rc != 0 {
		return fmt.Errorf("input: scroll rc=%d", rc)
	}
	return nil
}

// ScrollSmooth posts a series of smaller scroll events to approximate
// a single larger scroll. Looks more natural than a single big jump
// and avoids "overshoot" on momentum-scrolling apps.
func ScrollSmooth(ctx context.Context, dx, dy int, duration time.Duration, opts ...PostOption) error {
	if duration <= 0 {
		return Scroll(ctx, dx, dy, opts...)
	}
	steps := int(duration / (time.Second / 60))
	if steps < 2 {
		steps = 2
	}
	if steps > 300 {
		steps = 300
	}
	stepDur := duration / time.Duration(steps)

	// Distribute pixels across steps. Integer pixels per step plus a
	// carry so the total lands exactly on dx/dy.
	accX, accY := 0.0, 0.0
	sentX, sentY := 0, 0
	for i := 1; i <= steps; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		t := float64(i) / float64(steps)
		accX = float64(dx) * t
		accY = float64(dy) * t
		stepX := int(math.Round(accX)) - sentX
		stepY := int(math.Round(accY)) - sentY
		if stepX != 0 || stepY != 0 {
			if err := Scroll(ctx, stepX, stepY, opts...); err != nil {
				return err
			}
			sentX += stepX
			sentY += stepY
		}
		time.Sleep(stepDur)
	}
	return nil
}
