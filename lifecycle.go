package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	sessionHeartbeatTimeout = 90 * time.Second
	sessionShutdownGrace    = 6 * time.Second
	startupSessionTimeout   = 55 * time.Second
	serverShutdownTimeout   = 8 * time.Second
	maxSessionCount         = 128
	maxExpiredSessionCount  = 256
	expiredSessionTTL       = 6 * time.Hour
)

type application struct {
	paths                     appPaths
	profile                   *profileStore
	cache                     *responseCache
	logger                    *log.Logger
	logWriter                 *rotatingLogWriter
	server                    *http.Server
	listener                  net.Listener
	ctx                       context.Context
	cancel                    context.CancelFunc
	closing                   atomic.Bool
	shutdownOnce              sync.Once
	wg                        sync.WaitGroup
	sessions                  *sessionManager
	autoExit                  bool
	shutdownOnHeartbeatExpiry bool
	startupTimeout            time.Duration
}

type sessionEmptyReason uint8

const (
	sessionEmptyNone sessionEmptyReason = iota
	sessionEmptyExplicitClose
	sessionEmptyHeartbeatExpiry
	sessionEmptyExpiredTombstone
)

type sessionManager struct {
	mu               sync.Mutex
	sessions         map[string]time.Time
	expiredSessions  map[string]time.Time
	everOpened       bool
	lastTransition   time.Time
	emptyReason      sessionEmptyReason
	grace            time.Duration
	heartbeatTimeout time.Duration
	expiredTTL       time.Duration
	firstOpened      chan struct{}
	firstOpenOnce    sync.Once
}

func newSessionManager(graceValues ...time.Duration) *sessionManager {
	grace := sessionShutdownGrace
	heartbeatTimeout := sessionHeartbeatTimeout
	expiredTTL := expiredSessionTTL
	if len(graceValues) > 0 && graceValues[0] > 0 {
		grace = graceValues[0]
	}
	if len(graceValues) > 1 && graceValues[1] > 0 {
		heartbeatTimeout = graceValues[1]
	}
	if len(graceValues) > 2 && graceValues[2] > 0 {
		expiredTTL = graceValues[2]
	}
	return &sessionManager{
		sessions:         make(map[string]time.Time),
		expiredSessions:  make(map[string]time.Time),
		lastTransition:   time.Now(),
		grace:            grace,
		heartbeatTimeout: heartbeatTimeout,
		expiredTTL:       expiredTTL,
		firstOpened:      make(chan struct{}),
	}
}

func (s *sessionManager) addExpiredLocked(id string, now time.Time) {
	if !validSessionID(id) {
		return
	}
	if len(s.expiredSessions) >= maxExpiredSessionCount {
		oldestID := ""
		oldest := now
		for sessionID, expiresAt := range s.expiredSessions {
			if oldestID == "" || expiresAt.Before(oldest) {
				oldestID = sessionID
				oldest = expiresAt
			}
		}
		delete(s.expiredSessions, oldestID)
	}
	s.expiredSessions[id] = now.Add(s.expiredTTL)
}

func (s *sessionManager) pruneExpiredLocked(now time.Time) {
	hadActive := len(s.sessions) > 0
	hadTracked := hadActive || len(s.expiredSessions) > 0
	hadExpiredTombstones := len(s.expiredSessions) > 0
	for sessionID, expiresAt := range s.expiredSessions {
		if !now.Before(expiresAt) {
			delete(s.expiredSessions, sessionID)
		}
	}
	for sessionID, heartbeat := range s.sessions {
		if now.Sub(heartbeat) > s.heartbeatTimeout {
			delete(s.sessions, sessionID)
			s.addExpiredLocked(sessionID, now)
		}
	}
	if hadActive && len(s.sessions) == 0 {
		s.lastTransition = now
		s.emptyReason = sessionEmptyHeartbeatExpiry
	}
	if hadTracked && !hadActive && len(s.sessions) == 0 && len(s.expiredSessions) == 0 {
		s.lastTransition = now
		if hadExpiredTombstones {
			s.emptyReason = sessionEmptyExpiredTombstone
		} else {
			s.emptyReason = sessionEmptyHeartbeatExpiry
		}
	}
}

func (s *sessionManager) Open(id string) string {
	if !validSessionID(id) {
		id = randomSessionID()
	}
	now := time.Now()
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	delete(s.expiredSessions, id)
	if _, exists := s.sessions[id]; !exists && len(s.sessions) >= maxSessionCount {
		oldestID := ""
		oldestHeartbeat := now
		for sessionID, heartbeat := range s.sessions {
			if oldestID == "" || heartbeat.Before(oldestHeartbeat) {
				oldestID = sessionID
				oldestHeartbeat = heartbeat
			}
		}
		delete(s.sessions, oldestID)
		s.addExpiredLocked(oldestID, now)
	}
	s.sessions[id] = now
	s.everOpened = true
	s.lastTransition = now
	s.emptyReason = sessionEmptyNone
	s.mu.Unlock()
	s.firstOpenOnce.Do(func() { close(s.firstOpened) })
	return id
}

func (s *sessionManager) Heartbeat(id string) bool {
	if !validSessionID(id) {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	if _, ok := s.sessions[id]; !ok {
		return false
	}
	s.sessions[id] = now
	s.lastTransition = now
	return true
}

func (s *sessionManager) Close(id string) {
	if !validSessionID(id) {
		return
	}
	now := time.Now()
	s.mu.Lock()
	s.pruneExpiredLocked(now)
	_, active := s.sessions[id]
	_, expired := s.expiredSessions[id]
	if active {
		delete(s.sessions, id)
	}
	if expired {
		delete(s.expiredSessions, id)
	}
	// Only a session ID that was actually opened by this process may count as
	// an explicit close. Random/unknown IDs must never be able to stop the app.
	if (active || expired) && len(s.sessions) == 0 && len(s.expiredSessions) == 0 {
		s.lastTransition = now
		s.emptyReason = sessionEmptyExplicitClose
	}
	s.mu.Unlock()
}

func (s *sessionManager) ShouldShutdown(now time.Time, allowHeartbeatExpiry ...bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	if !s.everOpened || len(s.sessions) != 0 || now.Sub(s.lastTransition) < s.grace {
		return false
	}
	if s.emptyReason == sessionEmptyExplicitClose && len(s.expiredSessions) == 0 {
		return true
	}
	if s.emptyReason == sessionEmptyExpiredTombstone && len(s.expiredSessions) == 0 {
		return true
	}
	return len(allowHeartbeatExpiry) > 0 && allowHeartbeatExpiry[0] && s.emptyReason == sessionEmptyHeartbeatExpiry
}

func (s *sessionManager) FirstOpened() <-chan struct{} { return s.firstOpened }

func (s *sessionManager) HasEverOpened() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.everOpened
}

func (s *sessionManager) ActiveCount(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	return len(s.sessions)
}

func (s *sessionManager) ExpiredCount(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	return len(s.expiredSessions)
}

func validSessionID(id string) bool {
	if len(id) < 16 || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func randomSessionID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func (a *application) requestShutdown(reason string) {
	a.shutdownOnce.Do(func() {
		a.closing.Store(true)
		a.logger.Printf("запрошено завершение: %s", reason)
		a.cancel()
	})
}

func (a *application) monitorSessions() {
	defer a.wg.Done()
	interval := 2 * time.Second
	if a.sessions.grace < 4*time.Second {
		interval = a.sessions.grace / 2
		if interval < 100*time.Millisecond {
			interval = 100 * time.Millisecond
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var startupTimer *time.Timer
	var startupC <-chan time.Time
	if a.startupTimeout > 0 {
		startupTimer = time.NewTimer(a.startupTimeout)
		startupC = startupTimer.C
		defer stopTimer(startupTimer)
	}
	firstOpened := a.sessions.FirstOpened()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-firstOpened:
			firstOpened = nil
			if startupTimer != nil {
				stopTimer(startupTimer)
				startupTimer = nil
				startupC = nil
			}
		case <-startupC:
			startupC = nil
			startupTimer = nil
			// Opening a session and timer delivery may race. Re-check the
			// protected state before requesting shutdown.
			if !a.sessions.HasEverOpened() {
				a.requestShutdown("интерфейс не открылся за отведённое время")
				return
			}
		case now := <-ticker.C:
			if a.autoExit && a.sessions.ShouldShutdown(now, a.shutdownOnHeartbeatExpiry) {
				a.requestShutdown("последнее окно браузера закрыто")
				return
			}
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (a *application) shutdown() error {
	a.requestShutdown("штатное завершение")
	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	var shutdownErr error
	if a.server != nil {
		if err := a.server.Shutdown(ctx); err != nil && !errors.Is(err, context.Canceled) {
			shutdownErr = err
			_ = a.server.Close()
		}
	}
	wait := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(wait)
	}()
	select {
	case <-wait:
	case <-ctx.Done():
		if shutdownErr == nil {
			shutdownErr = ctx.Err()
		}
	}
	if err := a.profile.Flush(); err != nil {
		a.logger.Printf("сохранение профиля при завершении: %v", err)
		if shutdownErr == nil {
			shutdownErr = err
		}
	}
	a.cache.Clear()
	if a.logWriter != nil {
		_ = a.logWriter.Close()
	}
	return shutdownErr
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

func validateListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("некорректный адрес: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("приложение разрешает прослушивание только loopback-адреса")
	}
	return nil
}

type existingInstanceInfo struct {
	Found   bool
	Version string
	Release string
}

func probeExistingInstance(address string) existingInstanceInfo {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	response, err := client.Get("http://" + address + "/api/health")
	if err != nil {
		return existingInstanceInfo{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return existingInstanceInfo{}
	}
	var health struct {
		Status      string `json:"status"`
		Application string `json:"application"`
		Version     string `json:"version"`
		Release     string `json:"release"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4096))
	if err := decoder.Decode(&health); err != nil || health.Status != "ok" || health.Application != applicationID {
		return existingInstanceInfo{}
	}
	return existingInstanceInfo{Found: true, Version: health.Version, Release: health.Release}
}

func sameApplicationBuild(existing existingInstanceInfo) bool {
	return existing.Found && existing.Version == appVersion && existing.Release == releaseMarker
}

func (a *application) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "application": applicationID, "version": appVersion, "release": releaseMarker})
}

func (a *application) handleSessionOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		ID string `json:"id"`
	}
	if !decodeJSONRequest(w, r, &request, 4096) {
		return
	}
	id := a.sessions.Open(request.ID)
	writeJSON(w, map[string]any{"id": id, "heartbeatSeconds": 5})
}

func (a *application) handleSessionHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		ID string `json:"id"`
	}
	if !decodeJSONRequest(w, r, &request, 4096) {
		return
	}
	if !a.sessions.Heartbeat(request.ID) {
		http.Error(w, "Некорректная сессия.\n", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) handleSessionClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request struct {
		ID string `json:"id"`
	}
	if !decodeJSONRequest(w, r, &request, 4096) {
		return
	}
	a.sessions.Close(request.ID)
	w.WriteHeader(http.StatusNoContent)
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
			a.logger.Printf("сохранение профиля: %v", err)
			http.Error(w, "Не удалось сохранить профиль.\n", http.StatusInternalServerError)
			return
		}
		writeJSON(w, a.profile.Get())
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut)
	}
}

func processID() int { return os.Getpid() }
