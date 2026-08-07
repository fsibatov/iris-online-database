//go:build windows

package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

type fakeWindowsFileInfo struct{ attributes uint32 }

func (f fakeWindowsFileInfo) Name() string       { return "junction" }
func (f fakeWindowsFileInfo) Size() int64        { return 0 }
func (f fakeWindowsFileInfo) Mode() os.FileMode  { return os.ModeDir }
func (f fakeWindowsFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeWindowsFileInfo) IsDir() bool        { return true }
func (f fakeWindowsFileInfo) Sys() any {
	return &syscall.Win32FileAttributeData{FileAttributes: f.attributes}
}

func TestWindowsJunctionReparseAttributeRejected(t *testing.T) {
	if !fileInfoIsReparse(fakeWindowsFileInfo{attributes: fileAttributeReparsePoint}) {
		t.Fatal("FILE_ATTRIBUTE_REPARSE_POINT was not rejected")
	}
}
