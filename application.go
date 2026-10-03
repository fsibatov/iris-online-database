package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func newApplication() (*application, error) {
	paths, err := resolveAppPaths()
	if err != nil {
		return nil, errors.New("не удалось открыть папку настроек; проверьте права доступа и свободное место на диске")
	}

	executable, _ := os.Executable()
	bootstrapLogger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	runMaintenance(paths, executable, bootstrapLogger)

	logWriter, err := newRotatingLogWriter(filepath.Join(paths.Logs, "application.log"), 2<<20, 5)
	if err != nil {
		return nil, errors.New("не удалось открыть журнал приложения; проверьте права доступа и свободное место на диске")
	}

	logger := log.New(io.MultiWriter(os.Stderr, logWriter), "", log.Ldate|log.Ltime|log.Lmicroseconds)
	profile, err := newProfileStore(paths)
	if err != nil {
		_ = logWriter.Close()
		return nil, fmt.Errorf("профиль пользователя: %w", err)
	}
	if err := ensureLoaded(); err != nil {
		_ = logWriter.Close()
		return nil, errors.New("не удалось прочитать встроенную базу; скачайте приложение заново со страницы релизов")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &application{
		paths:     paths,
		profile:   profile,
		cache:     newResponseCache(128, 8<<20, 5*time.Minute),
		logger:    logger,
		logWriter: logWriter,
		ctx:       ctx,
		cancel:    cancel,
		updates:   newUpdateChecker(),
		community: newCommunityChecker(),
	}, nil
}
