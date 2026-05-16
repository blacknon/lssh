//go:build windows

package mux

import (
	"fmt"
	"unsafe"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/sys/windows"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard  = user32.NewProc("OpenClipboard")
	procCloseClipboard = user32.NewProc("CloseClipboard")
	procEmptyClipboard = user32.NewProc("EmptyClipboard")
	procSetClipboard   = user32.NewProc("SetClipboardData")
	procGlobalAlloc    = kernel32.NewProc("GlobalAlloc")
	procGlobalLock     = kernel32.NewProc("GlobalLock")
	procGlobalUnlock   = kernel32.NewProc("GlobalUnlock")
)

func setClipboardText(screen tcell.Screen, text string) error {
	if err := setWindowsClipboardText(text); err == nil {
		return nil
	}

	if screen == nil {
		return fmt.Errorf("clipboard unavailable")
	}
	screen.SetClipboard([]byte(text))
	return nil
}

func setWindowsClipboardText(text string) error {
	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return err
	}

	r1, _, errOpen := procOpenClipboard.Call(0)
	if r1 == 0 {
		return errOpen
	}
	defer procCloseClipboard.Call()

	r1, _, errEmpty := procEmptyClipboard.Call()
	if r1 == 0 {
		return errEmpty
	}

	size := uintptr(len(utf16) * 2)
	hMem, _, errAlloc := procGlobalAlloc.Call(gmemMoveable, size)
	if hMem == 0 {
		return errAlloc
	}

	ptr, _, errLock := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return errLock
	}

	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf16)), utf16)
	procGlobalUnlock.Call(hMem)

	r1, _, errSet := procSetClipboard.Call(cfUnicodeText, hMem)
	if r1 == 0 {
		return errSet
	}

	return nil
}
