//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#import <stdlib.h>
#import <string.h>

char* openFolderPanel() {
    @autoreleasepool {
        NSOpenPanel* panel = [NSOpenPanel openPanel];
        [panel setCanChooseFiles:NO];
        [panel setCanChooseDirectories:YES];
        [panel setAllowsMultipleSelection:NO];
        [panel setCanCreateDirectories:YES];
        NSInteger result = [panel runModal];
        if (result == NSModalResponseOK) {
            NSString* path = [[panel URL] path];
            const char* utf8 = [path UTF8String];
            char* copy = (char*)malloc(strlen(utf8) + 1);
            strcpy(copy, utf8);
            return copy;
        }
        return NULL;
    }
}
*/
import "C"
import (
	"runtime"
	"unsafe"

	"fyne.io/fyne/v2"
)

func chooseFolder(_ fyne.Window) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cpath := C.openFolderPanel()
	if cpath == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(cpath))
	return C.GoString(cpath), nil
}
