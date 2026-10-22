package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPendingDeleteRejectsWindowsReservedPunctuation(t *testing.T) {
	for _, name := range []string{"bad?name.bin", "bad|name.bin", "bad<name.bin", "bad>name.bin", "bad:name.bin", "bad\"name.bin", "bad*name.bin"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			queue := filepath.Join(root, "pending-delete.json")
			entry := filepath.Join(root, "Cache", name)
			data, err := json.Marshal(pendingDeleteList{Paths: []string{entry}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(queue, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := loadPendingDeleteList(queue); err == nil {
				t.Fatalf("invalid Windows path accepted: %+v", got.Paths)
			}
		})
	}
}

func TestPendingDeleteWindowsPunctuationRecoversBackupOnProcess(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(cache, "old.bin")
	if err := os.WriteFile(victim, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	queue := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "invalid?file.bin")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{victim}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	processPendingDeletes(queue, []string{cache}, "", log.New(io.Discard, "", 0))
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatalf("valid backup not replayed after invalid Windows path: %v", err)
	}
}

func TestPendingDeleteWindowsPunctuationPreservesBackupOnAppend(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "Cache")
	if err := os.Mkdir(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(cache, "old.bin")
	next := filepath.Join(cache, "next.bin")
	for _, path := range []string{old, next} {
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	queue := filepath.Join(root, "pending-delete.json")
	bad, _ := json.Marshal(pendingDeleteList{Paths: []string{filepath.Join(cache, "bad|file.bin")}})
	good, _ := json.Marshal(pendingDeleteList{Paths: []string{old}})
	if err := os.WriteFile(queue, bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(queue+".bak", good, 0o600); err != nil {
		t.Fatal(err)
	}
	appendPendingDeletes(queue, []string{next}, []string{cache}, "", log.New(io.Discard, "", 0))
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{old, next}) {
		t.Fatalf("backup queue lost: %+v, %v", got, err)
	}
	preserved, err := os.ReadFile(queue + ".bak")
	if err != nil || !bytes.Equal(preserved, good) {
		t.Fatalf("valid backup overwritten: %q, %v", preserved, err)
	}
}

func TestPendingDeleteAllowsOrdinaryPunctuationAndUnicode(t *testing.T) {
	root := t.TempDir()
	queue := filepath.Join(root, "pending-delete.json")
	entry := filepath.Join(root, "Cache", "Новый файл (1),_-.bin")
	data, _ := json.Marshal(pendingDeleteList{Paths: []string{entry}})
	if err := os.WriteFile(queue, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPendingDeleteList(queue)
	if err != nil || !reflect.DeepEqual(got.Paths, []string{entry}) {
		t.Fatalf("valid Windows filename rejected: %+v, %v", got, err)
	}
}
