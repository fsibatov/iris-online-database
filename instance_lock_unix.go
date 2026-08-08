//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type appInstanceLock struct {
	file *os.File
}

func acquireInstanceLock(paths appPaths) (*appInstanceLock, error) {
	lockPath := filepath.Join(paths.LocalRoot, "instance.lock")
	if !safeOwnedFilePath(lockPath) {
		return nil, errUnsafeDestructivePath
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, errInstanceAlreadyRunning
		}
		return nil, err
	}
	return &appInstanceLock{file: file}, nil
}

func (l *appInstanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return file.Close()
}
