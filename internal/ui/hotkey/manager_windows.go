//go:build windows

package hotkey

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type winPlatform struct {
	nextID uint32
	mu     sync.Mutex
	stop   func()
}

func newPlatform() (platform, error) {
	return &winPlatform{}, nil
}

type winMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct {
		x int32
		y int32
	}
}

const (
	modAlt     = 0x0001
	modControl = 0x0002
	modShift   = 0x0004
	modWin     = 0x0008
	wmHotkey   = 0x0312
	wmQuit     = 0x0012
)

var (
	user32                 = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessage         = user32.NewProc("GetMessageW")
	procPostThreadMessage  = user32.NewProc("PostThreadMessageW")
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
)

func (w *winPlatform) register(combo combination, callback func()) (func(), error) {
	key, err := winVirtualKey(combo.Key)
	if err != nil {
		return nil, err
	}
	modifiers := uint32(0)
	if combo.Ctrl {
		modifiers |= modControl
	}
	if combo.Alt {
		modifiers |= modAlt
	}
	if combo.Shift {
		modifiers |= modShift
	}
	if combo.Super {
		modifiers |= modWin
	}
	id := atomic.AddUint32(&w.nextID, 1)
	r, _, e := procRegisterHotKey.Call(0, uintptr(id), uintptr(modifiers), uintptr(key))
	if r == 0 {
		if e != syscall.Errno(0) {
			return nil, fmt.Errorf("RegisterHotKey failed: %v", e)
		}
		return nil, errors.New("RegisterHotKey failed")
	}

	ready := make(chan struct{})
	done := make(chan struct{})
	var threadID uint32

	go func() {
		runtime.LockOSThread()
		tid, _, _ := procGetCurrentThreadID.Call()
		threadID = uint32(tid)
		close(ready)
		msg := &winMsg{}
		for {
			ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(msg)), 0, 0, 0)
			if int32(ret) <= 0 {
				break
			}
			if msg.message == wmHotkey {
				callback()
			}
		}
		runtime.UnlockOSThread()
		close(done)
	}()

	<-ready

	cancel := func() {
		procUnregisterHotKey.Call(0, uintptr(id))
		procPostThreadMessage.Call(uintptr(threadID), wmQuit, 0, 0)
		<-done
	}

	return cancel, nil
}

func winVirtualKey(key string) (uint32, error) {
	if len(key) == 1 {
		ch := key[0]
		if ch >= 'a' && ch <= 'z' {
			ch = ch - 'a' + 'A'
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			return uint32(ch), nil
		}
	}
	switch key {
	case "enter":
		return 0x0D, nil
	case "space":
		return 0x20, nil
	case "tab":
		return 0x09, nil
	}
	return 0, fmt.Errorf("unsupported key %s", key)
}
