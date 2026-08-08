package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeLogRecord(t *testing.T, writer *rotatingLogWriter, record string) {
	t.Helper()
	if _, err := writer.Write([]byte(record)); err != nil {
		t.Fatalf("write log record %q: %v", record, err)
	}
}

func readLogFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestRotatingLogWriterNoBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Logs", "application.log")
	writer, err := newRotatingLogWriter(path, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	writeLogRecord(t, writer, "one\n")
	writeLogRecord(t, writer, "two\n")

	if got := readLogFile(t, path); got != "two\n" {
		t.Fatalf("current log = %q, want %q", got, "two\\n")
	}
	if _, err := os.Lstat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("backup unexpectedly exists: %v", err)
	}
}

func TestRotatingLogWriterSingleBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Logs", "application.log")
	writer, err := newRotatingLogWriter(path, 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	writeLogRecord(t, writer, "one\n")
	writeLogRecord(t, writer, "two\n")
	writeLogRecord(t, writer, "tri\n")

	if got := readLogFile(t, path); got != "tri\n" {
		t.Fatalf("current log = %q, want %q", got, "tri\\n")
	}
	if got := readLogFile(t, path+".1"); got != "two\n" {
		t.Fatalf("backup .1 = %q, want %q", got, "two\\n")
	}
	if _, err := os.Lstat(path + ".2"); !os.IsNotExist(err) {
		t.Fatalf("backup .2 unexpectedly exists: %v", err)
	}
}

func TestRotatingLogWriterMultipleBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Logs", "application.log")
	writer, err := newRotatingLogWriter(path, 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	writeLogRecord(t, writer, "one\n")
	writeLogRecord(t, writer, "two\n")
	writeLogRecord(t, writer, "tri\n")
	writeLogRecord(t, writer, "for\n")

	want := map[string]string{
		path:        "for\n",
		path + ".1": "tri\n",
		path + ".2": "two\n",
		path + ".3": "one\n",
	}
	for file, expected := range want {
		if got := readLogFile(t, file); got != expected {
			t.Fatalf("%s = %q, want %q", file, got, expected)
		}
	}
}

func TestRotatingLogReopenRejectsSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require Windows developer mode; reparse paths are covered by Windows path tests")
	}
	root := t.TempDir()
	logPath := filepath.Join(root, "application.log")
	outside := filepath.Join(t.TempDir(), "outside.log")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	writer, err := newRotatingLogWriter(logPath, 64, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.file.Close(); err != nil {
		t.Fatal(err)
	}
	writer.file = nil
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, logPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := writer.open(); !errors.Is(err, errUnsafeDestructivePath) {
		t.Fatalf("reopen through symlink error = %v, want unsafe path", err)
	}
	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("outside log target was modified: %q", data)
	}
}
