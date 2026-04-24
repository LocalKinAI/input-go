// input-go — CGEvent synthesis shim
//
// Exposes a minimal C ABI for Go purego callers to synthesize mouse and
// keyboard events via CGEvent. All calls run on the caller thread; event
// posting is synchronous (CGEventPost returns immediately once the event
// is enqueued into the HID system).
//
// Permission note: macOS 10.15+ requires the invoking binary to be listed
// in System Settings → Privacy & Security → Accessibility for CGEvent*
// to take effect. Without the permission the call still returns success
// but the events have no visible effect. Callers that care must check
// AXIsProcessTrusted() themselves — see input_ax_trusted below.
//
// Thread safety: CGEvent APIs are safe to call from any thread.

#import <Foundation/Foundation.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>

// ─── Exported C ABI ──────────────────────────────────────────

#ifdef __cplusplus
extern "C" {
#endif

// Probe whether the current process has Accessibility permission.
// Returns 1 if granted, 0 if not. Does NOT prompt — pass prompt=1 to
// show the system dialog on first call.
int32_t input_ax_trusted(int32_t prompt);

// Return current cursor position in global screen coordinates.
// Writes doubles through x_out / y_out.
int32_t input_cursor_position(double *x_out, double *y_out);

// Post a mouse event. button: 0=left, 1=right, 2=other.
// kind: 0=move, 1=down, 2=up, 3=drag.
// If kind==move or kind==drag, the event carries the new cursor position.
// clicks: click count (1=single, 2=double, 3=triple). Only meaningful
// for down/up events.
int32_t input_mouse_event(double x, double y,
                          int32_t button, int32_t kind,
                          int32_t clicks);

// Post a scroll wheel event. dy is vertical (positive = scroll up),
// dx is horizontal (positive = scroll right). Units are "lines" (pixel
// scale) per CGEventCreateScrollWheelEvent.
int32_t input_scroll(int32_t dx, int32_t dy);

// Post a keyboard event. keycode is a macOS virtual keycode
// (kVK_ANSI_A etc., see <Carbon/HIToolbox/Events.h>).
// down: 1=key down, 0=key up.
// flags: CGEventFlags bitmask (kCGEventFlagMaskCommand | ...).
int32_t input_key(int32_t keycode, int32_t down, uint64_t flags);

// Type a UTF-8 string by synthesizing keyboard-event-from-unicode
// events. Does not go through a physical keycode — works for any
// character regardless of the current keyboard layout.
int32_t input_type_unicode(const char *utf8, int32_t utf8_len);

// Sleep for ms milliseconds on the caller thread. Convenience so Go
// code can pace events without round-tripping through Go's time
// package (useful inside tight CGEvent sequences).
int32_t input_sleep_ms(int32_t ms);

// Return the main screen bounds in global coordinates.
// Writes doubles through width_out / height_out.
int32_t input_screen_size(double *width_out, double *height_out);

#ifdef __cplusplus
}
#endif

// ─── Implementation ──────────────────────────────────────────

int32_t input_ax_trusted(int32_t prompt) {
    if (prompt) {
        NSDictionary *opts = @{
            (__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES
        };
        return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts) ? 1 : 0;
    }
    return AXIsProcessTrusted() ? 1 : 0;
}

int32_t input_cursor_position(double *x_out, double *y_out) {
    if (!x_out || !y_out) return -1;
    CGEventRef ev = CGEventCreate(NULL);
    if (!ev) return -1;
    CGPoint p = CGEventGetLocation(ev);
    CFRelease(ev);
    *x_out = p.x;
    *y_out = p.y;
    return 0;
}

int32_t input_screen_size(double *w_out, double *h_out) {
    if (!w_out || !h_out) return -1;
    CGDirectDisplayID main = CGMainDisplayID();
    size_t w = CGDisplayPixelsWide(main);
    size_t h = CGDisplayPixelsHigh(main);
    *w_out = (double)w;
    *h_out = (double)h;
    return 0;
}

// Map button enum → (CGMouseButton, down/up event type, drag event type).
static void mouse_event_types(int32_t button,
                              CGMouseButton *cgBtn,
                              CGEventType *downType,
                              CGEventType *upType,
                              CGEventType *dragType) {
    switch (button) {
        case 1: // right
            *cgBtn = kCGMouseButtonRight;
            *downType = kCGEventRightMouseDown;
            *upType   = kCGEventRightMouseUp;
            *dragType = kCGEventRightMouseDragged;
            break;
        case 2: // other
            *cgBtn = kCGMouseButtonCenter;
            *downType = kCGEventOtherMouseDown;
            *upType   = kCGEventOtherMouseUp;
            *dragType = kCGEventOtherMouseDragged;
            break;
        case 0: // left (default)
        default:
            *cgBtn = kCGMouseButtonLeft;
            *downType = kCGEventLeftMouseDown;
            *upType   = kCGEventLeftMouseUp;
            *dragType = kCGEventLeftMouseDragged;
            break;
    }
}

int32_t input_mouse_event(double x, double y,
                          int32_t button, int32_t kind,
                          int32_t clicks) {
    CGMouseButton cgBtn;
    CGEventType downType, upType, dragType;
    mouse_event_types(button, &cgBtn, &downType, &upType, &dragType);

    CGPoint p = CGPointMake(x, y);
    CGEventType type;
    switch (kind) {
        case 0: type = kCGEventMouseMoved; break;
        case 1: type = downType; break;
        case 2: type = upType;   break;
        case 3: type = dragType; break;
        default: return -1;
    }

    CGEventRef ev = CGEventCreateMouseEvent(NULL, type, p, cgBtn);
    if (!ev) return -1;
    if (clicks > 1 && (kind == 1 || kind == 2)) {
        CGEventSetIntegerValueField(ev, kCGMouseEventClickState, (int64_t)clicks);
    }
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
    return 0;
}

int32_t input_scroll(int32_t dx, int32_t dy) {
    // Pixel-unit scroll: axis2 = horizontal, axis1 = vertical.
    CGEventRef ev = CGEventCreateScrollWheelEvent(
        NULL, kCGScrollEventUnitPixel, 2,
        (int32_t)dy, (int32_t)dx);
    if (!ev) return -1;
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
    return 0;
}

int32_t input_key(int32_t keycode, int32_t down, uint64_t flags) {
    CGEventRef ev = CGEventCreateKeyboardEvent(
        NULL, (CGKeyCode)keycode, down != 0);
    if (!ev) return -1;
    if (flags != 0) {
        CGEventSetFlags(ev, (CGEventFlags)flags);
    }
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
    return 0;
}

int32_t input_type_unicode(const char *utf8, int32_t utf8_len) {
    if (!utf8 || utf8_len <= 0) return 0;
    NSString *s = [[NSString alloc] initWithBytes:utf8
                                           length:(NSUInteger)utf8_len
                                         encoding:NSUTF8StringEncoding];
    if (!s) return -1;

    // Strategy: one down+up pair per character, each event tagged with
    // the unicode codepoint(s) via CGEventKeyboardSetUnicodeString.
    // This bypasses keycode/layout issues entirely.
    NSUInteger len = [s length];
    for (NSUInteger i = 0; i < len; ) {
        // Extract the next grapheme range (handles surrogate pairs / emoji
        // correctly — a single "typed character" may span multiple UTF-16
        // units).
        NSRange cluster = [s rangeOfComposedCharacterSequenceAtIndex:i];
        NSString *ch = [s substringWithRange:cluster];
        NSUInteger chlen = [ch length];
        unichar buf[8];
        if (chlen > 8) chlen = 8;
        [ch getCharacters:buf range:NSMakeRange(0, chlen)];

        CGEventRef down = CGEventCreateKeyboardEvent(NULL, 0, true);
        CGEventRef up   = CGEventCreateKeyboardEvent(NULL, 0, false);
        if (!down || !up) {
            if (down) CFRelease(down);
            if (up)   CFRelease(up);
            return -1;
        }
        CGEventKeyboardSetUnicodeString(down, chlen, buf);
        CGEventKeyboardSetUnicodeString(up,   chlen, buf);
        CGEventPost(kCGHIDEventTap, down);
        CGEventPost(kCGHIDEventTap, up);
        CFRelease(down);
        CFRelease(up);

        i = cluster.location + cluster.length;
    }
    return 0;
}

int32_t input_sleep_ms(int32_t ms) {
    if (ms <= 0) return 0;
    struct timespec ts;
    ts.tv_sec  = ms / 1000;
    ts.tv_nsec = (long)(ms % 1000) * 1000000L;
    nanosleep(&ts, NULL);
    return 0;
}
