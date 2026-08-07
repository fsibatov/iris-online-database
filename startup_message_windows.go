//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func showStartupMessage(message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	text, err1 := syscall.UTF16PtrFromString(message)
	title, err2 := syscall.UTF16PtrFromString("Iris Online")
	if err1 != nil || err2 != nil {
		return
	}
	const mbOKIconError = 0x00000000 | 0x00000010
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), uintptr(mbOKIconError))
}
