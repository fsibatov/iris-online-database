//go:build windows

package main

import (
	"errors"
	"path/filepath"
	"syscall"
)

type appInstanceLock struct {
	handle syscall.Handle
}

func acquireInstanceLock(paths appPaths) (*appInstanceLock, error) {
	lockPath := filepath.Join(paths.LocalRoot, "instance.lock")
	if !safeOwnedFilePath(lockPath) {
		return nil, errUnsafeDestructivePath
	}
	pathPtr, err := syscall.UTF16PtrFromString(lockPath)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // no sharing: the open handle is the cross-process lock
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		if errors.Is(err, syscall.Errno(32)) {
			return nil, errInstanceAlreadyRunning
		}
		return nil, err
	}
	return &appInstanceLock{handle: handle}, nil
}

func (l *appInstanceLock) Close() error {
	if l == nil || l.handle == 0 || l.handle == syscall.InvalidHandle {
		return nil
	}
	handle := l.handle
	l.handle = 0
	return syscall.CloseHandle(handle)
}
