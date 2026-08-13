package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
)

type application struct {
	paths        appPaths
	profile      *profileStore
	cache        *responseCache
	logger       *log.Logger
	logWriter    *rotatingLogWriter
	ctx          context.Context
	cancel       context.CancelFunc
	closing      atomic.Bool
	shutdownOnce sync.Once
	updates      *updateChecker
	community    *communityChecker
}

func (a *application) requestShutdown(reason string) {
	a.shutdownOnce.Do(func() {
		a.closing.Store(true)
		if a.logger != nil {
			a.logger.Printf("запрошено завершение: %s", reason)
		}
		if a.cancel != nil {
			a.cancel()
		}
	})
}

func (a *application) shutdown() error {
	a.requestShutdown("штатное завершение")
	var shutdownErr error
	if a.profile != nil {
		if err := a.profile.Flush(); err != nil {
			shutdownErr = err
			if a.logger != nil {
				a.logger.Printf("сохранение профиля при завершении: %v", err)
			}
		}
	}
	if a.cache != nil {
		a.cache.Clear()
	}
	if a.logWriter != nil {
		_ = a.logWriter.Close()
	}
	return shutdownErr
}

func (a *application) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, map[string]any{
		"status":       "ok",
		"application":  applicationID,
		"version":      appVersion,
		"release":      releaseMarker,
		"architecture": "desktop-webview2",
	})
}

func (a *application) handleUserData(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.profile.Get())
	case http.MethodPut:
		if a.closing.Load() {
			http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
			return
		}
		var profile userProfile
		if !decodeJSONRequest(w, r, &profile, 1<<20) {
			return
		}
		if err := a.profile.Replace(profile); err != nil {
			if a.logger != nil {
				a.logger.Printf("сохранение профиля: %v", err)
			}
			http.Error(w, "Не удалось сохранить профиль.\n", http.StatusInternalServerError)
			return
		}
		writeJSON(w, a.profile.Get())
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}
