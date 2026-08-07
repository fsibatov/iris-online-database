//go:build windows

package main

import (
	"os"
	"syscall"
)

const fileAttributeReparsePoint = 0x00000400

func fileInfoIsReparse(info os.FileInfo) bool {
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&fileAttributeReparsePoint != 0
}
