//go:build darwin

package hotkey

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static CFMachPortRef eventTap = NULL;
static CFRunLoopSourceRef runLoopSource = NULL;
extern void goHandleHotkey(uint64_t flags, uint64_t keycode);

static CGEventRef tapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *refcon) {
        if (type == kCGEventKeyDown) {
                uint64_t keycode = CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
                uint64_t flags = CGEventGetFlags(event);
                goHandleHotkey(flags, keycode);
        }
        return event;
}

static int startTap() {
        if (eventTap != NULL) {
                return 0;
        }
        eventTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault,
                                     CGEventMaskBit(kCGEventKeyDown), tapCallback, NULL);
        if (!eventTap) {
                return -1;
        }
        runLoopSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, eventTap, 0);
        CFRunLoopAddSource(CFRunLoopGetCurrent(), runLoopSource, kCFRunLoopCommonModes);
        CGEventTapEnable(eventTap, true);
        return 0;
}

static void stopTap() {
        if (eventTap) {
                CGEventTapEnable(eventTap, false);
                if (runLoopSource) {
                        CFRunLoopRemoveSource(CFRunLoopGetCurrent(), runLoopSource, kCFRunLoopCommonModes);
                        CFRelease(runLoopSource);
                        runLoopSource = NULL;
                }
                CFRelease(eventTap);
                eventTap = NULL;
        }
}
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

type macPlatform struct {
	mu       sync.Mutex
	combo    combination
	callback func()
	runLoop  C.CFRunLoopRef
	running  bool
}

var macInstance *macPlatform
var macOnce sync.Once

func newPlatform() (platform, error) {
	macOnce.Do(func() {
		macInstance = &macPlatform{}
	})
	return macInstance, nil
}

func (m *macPlatform) register(combo combination, callback func()) (func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := macKeyCodes[combo.Key]; !ok {
		return nil, fmt.Errorf("unsupported key %s", combo.Key)
	}
	m.combo = combo
	m.callback = callback
	if !m.running {
		ready := make(chan error, 1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			if C.startTap() != 0 {
				ready <- errors.New("failed to start event tap")
				return
			}
			m.mu.Lock()
			m.runLoop = C.CFRunLoopGetCurrent()
			m.running = true
			m.mu.Unlock()
			close(ready)
			C.CFRunLoopRun()
			m.mu.Lock()
			m.running = false
			m.runLoop = nil
			m.mu.Unlock()
		}()
		if err := <-ready; err != nil {
			return nil, err
		}
	}
	cancel := func() {
		m.mu.Lock()
		if m.running && m.runLoop != 0 {
			C.CFRunLoopStop(m.runLoop)
			C.stopTap()
			m.running = false
			m.runLoop = 0
		}
		m.mu.Unlock()
	}
	return cancel, nil
}

//export goHandleHotkey
func goHandleHotkey(flags C.uint64_t, keycode C.uint64_t) {
	if macInstance == nil {
		return
	}
	macInstance.handle(uint64(flags), uint64(keycode))
}

func (m *macPlatform) handle(flags uint64, keycode uint64) {
	m.mu.Lock()
	combo := m.combo
	cb := m.callback
	m.mu.Unlock()
	expected, ok := macKeyCodes[combo.Key]
	if !ok || expected != keycode {
		return
	}
	if combo.Ctrl && flags&uint64(C.kCGEventFlagMaskControl) == 0 {
		return
	}
	if combo.Alt && flags&uint64(C.kCGEventFlagMaskAlternate) == 0 {
		return
	}
	if combo.Shift && flags&uint64(C.kCGEventFlagMaskShift) == 0 {
		return
	}
	if combo.Super && flags&uint64(C.kCGEventFlagMaskCommand) == 0 {
		return
	}
	if cb != nil {
		cb()
	}
}

var macKeyCodes = map[string]uint64{
	"a":      0,
	"s":      1,
	"d":      2,
	"f":      3,
	"h":      4,
	"g":      5,
	"z":      6,
	"x":      7,
	"c":      8,
	"v":      9,
	"b":      11,
	"q":      12,
	"w":      13,
	"e":      14,
	"r":      15,
	"y":      16,
	"t":      17,
	"1":      18,
	"2":      19,
	"3":      20,
	"4":      21,
	"6":      22,
	"5":      23,
	"=":      24,
	"9":      25,
	"7":      26,
	"-":      27,
	"8":      28,
	"0":      29,
	"]":      30,
	"o":      31,
	"u":      32,
	"[":      33,
	"i":      34,
	"p":      35,
	"l":      37,
	"j":      38,
	"\u0027": 39,
	"k":      40,
	";":      41,
	",":      43,
	".":      47,
	"/":      44,
	"n":      45,
	"m":      46,
	"space":  49,
	"return": 36,
}
