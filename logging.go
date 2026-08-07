package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type rotatingLogWriter struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	size     int64
	maxBytes int64
	backups  int
}

func newRotatingLogWriter(path string, maxBytes int64, backups int) (*rotatingLogWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if !safeOwnedFilePath(path) {
		return nil, errUnsafeDestructivePath
	}
	writer := &rotatingLogWriter{path: path, maxBytes: maxBytes, backups: backups}
	if err := writer.open(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingLogWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, io.ErrClosedPipe
	}
	if w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingLogWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Sync()
		_ = w.file.Close()
		w.file = nil
	}
	for i := w.backups - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", w.path, i)
		to := fmt.Sprintf("%s.%d", w.path, i+1)
		_ = os.Remove(to)
		_ = os.Rename(from, to)
	}
	if w.backups > 0 {
		_ = os.Remove(w.path + ".1")
		_ = os.Rename(w.path, w.path+".1")
	} else {
		_ = os.Remove(w.path)
	}
	return w.open()
}

func (w *rotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	_ = w.file.Sync()
	err := w.file.Close()
	w.file = nil
	return err
}
