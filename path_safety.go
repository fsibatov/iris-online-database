package main

import (
	"errors"
	"os"
	"path/filepath"
)

var errUnsafeDestructivePath = errors.New("unsafe destructive path")

func safeMaintenanceRoot(root string) (string, bool) {
	if root == "" {
		return "", false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	rootAbs = filepath.Clean(rootAbs)
	info, err := os.Lstat(rootAbs)
	if err != nil || !info.IsDir() {
		return "", false
	}
	if !pathComponentsArePlain(rootAbs) {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", false
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil || !sameFilePath(rootAbs, resolvedAbs) {
		return "", false
	}
	return rootAbs, true
}

func pathComponentsArePlain(path string) bool {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err != nil || fileInfoIsReparse(info) {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return true
		}
		current = parent
	}
}

func validatedDestructivePath(path, root, currentExecutable string) (string, bool) {
	if path == "" || root == "" {
		return "", false
	}
	rootAbs, ok := safeMaintenanceRoot(root)
	if !ok {
		return "", false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	pathAbs = filepath.Clean(pathAbs)
	if !insideRoot(pathAbs, rootAbs) || sameFilePath(pathAbs, rootAbs) {
		return "", false
	}
	if currentExecutable != "" && (sameFilePath(pathAbs, currentExecutable) || insideRoot(currentExecutable, pathAbs)) {
		return "", false
	}

	if !pathComponentsArePlain(pathAbs) {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(pathAbs)
	if err != nil {
		return "", false
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil || !insideRoot(resolvedAbs, rootAbs) || sameFilePath(resolvedAbs, rootAbs) {
		return "", false
	}
	return pathAbs, true
}

func destructiveRootForPath(path string, roots []string, currentExecutable string) (string, string, bool) {
	for _, root := range roots {
		if validated, ok := validatedDestructivePath(path, root, currentExecutable); ok {
			return validated, root, true
		}
	}
	return "", "", false
}

func safeOwnedFilePath(path string) bool {
	if path == "" {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	parent := filepath.Dir(pathAbs)
	if _, ok := safeMaintenanceRoot(parent); !ok {
		return false
	}
	info, err := os.Lstat(pathAbs)
	if err == nil {
		return !fileInfoIsReparse(info) && !info.IsDir()
	}
	return os.IsNotExist(err)
}

func safeRemove(path, root, currentExecutable string) error {
	validated, ok := validatedDestructivePath(path, root, currentExecutable)
	if !ok {
		return errUnsafeDestructivePath
	}
	return os.Remove(validated)
}

func safeRemoveAll(path, root, currentExecutable string) error {
	validated, ok := validatedDestructivePath(path, root, currentExecutable)
	if !ok {
		return errUnsafeDestructivePath
	}
	return os.RemoveAll(validated)
}
