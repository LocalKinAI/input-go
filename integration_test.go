//go:build integration

package input_test

// Integration tests — require Accessibility permission AND an interactive
// desktop session. These are NOT run in CI. Execute manually with:
//
//	make test-integration
//
// or:
//
//	go test -tags integration ./...
//
// On a fresh binary, the first run will pop the Accessibility prompt.
// Grant permission, then rerun.

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/LocalKinAI/input-go"
)

func TestLoad(t *testing.T) {
	if err := input.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if input.ResolvedDylibPath() == "" {
		t.Fatal("ResolvedDylibPath is empty after Load")
	}
}

func TestCursorPositionReadable(t *testing.T) {
	x, y, err := input.CursorPosition()
	if err != nil {
		t.Fatalf("CursorPosition: %v", err)
	}
	if math.IsNaN(x) || math.IsNaN(y) {
		t.Errorf("CursorPosition returned NaN: (%v, %v)", x, y)
	}
}

func TestScreenSizePositive(t *testing.T) {
	w, h, err := input.ScreenSize()
	if err != nil {
		t.Fatalf("ScreenSize: %v", err)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("ScreenSize returned non-positive: (%v, %v)", w, h)
	}
}

func TestMoveChangesCursorPosition(t *testing.T) {
	if !input.Trusted() {
		t.Skip("Accessibility permission not granted — skipping (grant in System Settings → Privacy & Security → Accessibility)")
	}
	ctx := context.Background()

	// Save original, restore at end.
	origX, origY, err := input.CursorPosition()
	if err != nil {
		t.Fatalf("initial CursorPosition: %v", err)
	}
	t.Cleanup(func() {
		_ = input.Move(ctx, origX, origY)
	})

	// Move to a known spot well away from origin.
	w, h, err := input.ScreenSize()
	if err != nil {
		t.Fatalf("ScreenSize: %v", err)
	}
	targetX, targetY := w/2, h/2
	if err := input.Move(ctx, targetX, targetY); err != nil {
		t.Fatalf("Move: %v", err)
	}
	// Small pause to let the event propagate.
	time.Sleep(50 * time.Millisecond)

	gotX, gotY, err := input.CursorPosition()
	if err != nil {
		t.Fatalf("post-move CursorPosition: %v", err)
	}
	if math.Abs(gotX-targetX) > 2 || math.Abs(gotY-targetY) > 2 {
		t.Errorf("cursor did not move: want (%v, %v), got (%v, %v)", targetX, targetY, gotX, gotY)
	}
}

func TestMoveSmoothCompletes(t *testing.T) {
	if !input.Trusted() {
		t.Skip("Accessibility permission not granted")
	}
	ctx := context.Background()

	origX, origY, _ := input.CursorPosition()
	t.Cleanup(func() { _ = input.Move(ctx, origX, origY) })

	if err := input.MoveSmooth(ctx, 400, 300, 100*time.Millisecond); err != nil {
		t.Fatalf("MoveSmooth: %v", err)
	}
	gotX, gotY, _ := input.CursorPosition()
	if math.Abs(gotX-400) > 2 || math.Abs(gotY-300) > 2 {
		t.Errorf("smooth move landed off-target: got (%v, %v)", gotX, gotY)
	}
}

func TestScrollDoesNotError(t *testing.T) {
	if !input.Trusted() {
		t.Skip("Accessibility permission not granted")
	}
	ctx := context.Background()
	if err := input.Scroll(ctx, 0, -3); err != nil {
		t.Fatalf("Scroll: %v", err)
	}
}

func TestTypeDoesNotError(t *testing.T) {
	// Note: this actually types into the focused app. Run from an
	// empty TextEdit or similar to avoid surprises.
	if !input.Trusted() {
		t.Skip("Accessibility permission not granted")
	}
	if testing.Short() {
		t.Skip("skip in -short mode (actually types text)")
	}
	ctx := context.Background()
	if err := input.Type(ctx, ""); err != nil {
		t.Fatalf("Type empty string: %v", err)
	}
}

func TestRequireTrustReturnsSentinel(t *testing.T) {
	err := input.RequireTrust()
	if err != nil && err != input.ErrNotTrusted {
		t.Errorf("RequireTrust returned unexpected error: %v", err)
	}
}
