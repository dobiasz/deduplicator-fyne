//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

int md_get_window_origin(double* x, double* y) {
    @autoreleasepool {
        NSWindow* window = [NSApp keyWindow];
        if (window == nil) {
            window = [NSApp mainWindow];
        }
        if (window == nil) {
            return 0;
        }

        NSRect frame = [window frame];
        *x = frame.origin.x;
        *y = frame.origin.y;
        return 1;
    }
}

void md_set_window_origin(double x, double y) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow* window = [NSApp keyWindow];
        if (window == nil) {
            window = [NSApp mainWindow];
        }
        if (window == nil) {
            return;
        }

        NSPoint point = NSMakePoint(x, y);
        [window setFrameOrigin:point];
    });
}
*/
import "C"

import (
	"time"

	"fyne.io/fyne/v2"
)

func restoreWindowPosition(a fyne.App, _ fyne.Window) {
    prefs := a.Preferences()
    x := prefs.FloatWithFallback(windowXPrefKey, -1)
    y := prefs.FloatWithFallback(windowYPrefKey, -1)
    if x < 0 || y < 0 {
        return
    }

    go func() {
        // Wait until the native window is created and shown.
        time.Sleep(250 * time.Millisecond)
        C.md_set_window_origin(C.double(x), C.double(y))
    }()
}

func saveWindowPosition(a fyne.App, _ fyne.Window) {
    var x C.double
    var y C.double
    ok := C.md_get_window_origin(&x, &y)
    if ok == 0 {
        return
    }

    prefs := a.Preferences()
    prefs.SetFloat(windowXPrefKey, float64(x))
    prefs.SetFloat(windowYPrefKey, float64(y))
}
