package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	profileSchemaVersion = 1
	cacheMaxBytes        = int64(128 << 20)
	cacheMaxAge          = 30 * 24 * time.Hour
	tempMaxAge           = 24 * time.Hour
	updateMaxAge         = 7 * 24 * time.Hour
	logMaxAge            = 30 * 24 * time.Hour
	maxProfileBytes      = int64(2 << 20)
	maxPendingListBytes  = int64(1 << 20)
)

type appPaths struct {
	RoamingRoot   string
	LocalRoot     string
	UserData      string
	Backups       string
	Cache         string
	Temp          string
	Logs          string
	Updates       string
	Versions      string
	WebViewData   string
	PendingDelete string
	Profile       string
}

func resolveAppPaths() (appPaths, error) {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return appPaths{}, fmt.Errorf("каталог настроек: %w", err)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return appPaths{}, fmt.Errorf("локальный каталог: %w", err)
	}
	p := appPaths{
		RoamingRoot: filepath.Join(configRoot, "IrisOnlineDatabase"),
		LocalRoot:   filepath.Join(cacheRoot, "IrisOnlineDatabase"),
	}
	p.UserData = filepath.Join(p.RoamingRoot, "UserData")
	p.Backups = filepath.Join(p.UserData, "Backups")
	p.Cache = filepath.Join(p.LocalRoot, "Cache")
	p.Temp = filepath.Join(p.LocalRoot, "Temp")
	p.Logs = filepath.Join(p.LocalRoot, "Logs")
	p.Updates = filepath.Join(p.LocalRoot, "Updates")
	p.Versions = filepath.Join(p.LocalRoot, "Versions")
	p.WebViewData = filepath.Join(p.LocalRoot, "WebView2")
	p.PendingDelete = filepath.Join(p.LocalRoot, "pending-delete.json")
	p.Profile = filepath.Join(p.UserData, "profile.json")
	for _, dir := range []string{p.UserData, p.Backups, p.Cache, p.Temp, p.Logs, p.Updates, p.Versions, p.WebViewData} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return appPaths{}, fmt.Errorf("создание %s: %w", dir, err)
		}
		if _, ok := safeMaintenanceRoot(dir); !ok {
			return appPaths{}, fmt.Errorf("небезопасный каталог приложения: %s", dir)
		}
	}
	return p, nil
}

type userSettings struct {
	Server string `json:"server"`
	Theme  string `json:"theme"`
	View   string `json:"view"`
}

type recentViewEntry struct {
	Type   string `json:"type"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Meta   string `json:"meta,omitempty"`
	Server string `json:"server,omitempty"`
}

type userProfile struct {
	SchemaVersion  int                        `json:"schemaVersion"`
	UpdatedAt      time.Time                  `json:"updatedAt"`
	Migrated       bool                       `json:"migrated"`
	Settings       userSettings               `json:"settings"`
	ItemFilters    map[string]string          `json:"itemFilters"`
	MonsterFilters map[string]string          `json:"monsterFilters"`
	Favorites      []string                   `json:"favorites"`
	History        []string                   `json:"history"`
	RecentlyViewed []recentViewEntry          `json:"recentlyViewed"`
	Extra          map[string]json.RawMessage `json:"-"`
}

var knownProfileKeys = map[string]struct{}{
	"schemaVersion": {}, "updatedAt": {}, "migrated": {}, "settings": {},
	"itemFilters": {}, "monsterFilters": {}, "favorites": {}, "history": {}, "recentlyViewed": {},
}

type userProfileWire userProfile

// encoding/json accepts case-insensitive aliases for struct fields. Treat
// those aliases as reserved too, so Extra cannot shadow canonical fields.
func canonicalProfileKey(key string) (string, bool) {
	if _, known := knownProfileKeys[key]; known {
		return key, true
	}
	for canonical := range knownProfileKeys {
		if strings.EqualFold(key, canonical) {
			return canonical, true
		}
	}
	return "", false
}

// Nested settings fields are decoded case-insensitively by encoding/json.
// Reject multiple spellings of one preference before decoding the struct.
func rejectConflictingSettingsAliases(data json.RawMessage) error {
	// encoding/json accepts null for structs and strings, silently clearing
	// preferences. Reject explicit nulls so a damaged disk profile can fall
	// back to its valid backup instead of replacing user settings.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("null profile settings")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	seen := make(map[string]string, 3)
	for key, value := range fields {
		for _, canonical := range [...]string{"server", "theme", "view"} {
			if !strings.EqualFold(key, canonical) {
				continue
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("null profile setting %q", key)
			}
			if previous, exists := seen[canonical]; exists {
				return fmt.Errorf("conflicting settings fields %q and %q", previous, key)
			}
			seen[canonical] = key
			break
		}
	}
	return nil
}

func (p *userProfile) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// Two differently cased names can write the same struct field. Reject
	// the ambiguous profile rather than making its value depend on JSON order.
	seen := make(map[string]string, len(knownProfileKeys))
	for key := range raw {
		canonical, known := canonicalProfileKey(key)
		if !known {
			continue
		}
		if previous, exists := seen[canonical]; exists {
			return fmt.Errorf("conflicting profile fields %q and %q", previous, key)
		}
		seen[canonical] = key
		if canonical == "settings" {
			if err := rejectConflictingSettingsAliases(raw[key]); err != nil {
				return err
			}
		}
	}
	var wire userProfileWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	extra := make(map[string]json.RawMessage)
	for key, value := range raw {
		if _, known := canonicalProfileKey(key); known {
			continue
		}
		extra[key] = append(json.RawMessage(nil), value...)
	}
	*p = userProfile(wire)
	p.Extra = extra
	return nil
}

func (p userProfile) MarshalJSON() ([]byte, error) {
	wire := userProfileWire(p)
	wire.Extra = nil
	knownData, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(knownData, &raw); err != nil {
		return nil, err
	}
	for key, value := range p.Extra {
		if _, known := canonicalProfileKey(key); known || !json.Valid(value) {
			continue
		}
		raw[key] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(raw)
}

type profileStore struct {
	mu      sync.RWMutex
	path    string
	backup  string
	profile userProfile
}

func newProfileStore(paths appPaths) (*profileStore, error) {
	ps := &profileStore{path: paths.Profile, backup: filepath.Join(paths.Backups, "profile.json.bak")}
	profile, err := loadProfileFile(ps.path)
	if err != nil {
		profile, err = loadProfileFile(ps.backup)
	}
	if err != nil {
		profile = defaultProfile()
	}
	profile = sanitizeProfile(profile)
	ps.profile = profile
	return ps, nil
}

func defaultProfile() userProfile {
	return userProfile{
		SchemaVersion:  profileSchemaVersion,
		UpdatedAt:      time.Now().UTC(),
		Migrated:       true,
		Settings:       userSettings{Server: "kiss", Theme: "dark", View: "list"},
		ItemFilters:    map[string]string{},
		MonsterFilters: map[string]string{},
		Favorites:      []string{},
		History:        []string{},
		RecentlyViewed: []recentViewEntry{},
		Extra:          map[string]json.RawMessage{},
	}
}

func loadProfileFile(path string) (userProfile, error) {
	if !safeOwnedFilePath(path) {
		return userProfile{}, errors.New("небезопасный путь профиля")
	}
	data, err := readLimitedFile(path, maxProfileBytes)
	if err != nil {
		return userProfile{}, err
	}
	// Apply the same duplicate-key policy to disk as to API requests. A
	// malformed primary profile must not silently override preferences or
	// prevent recovery from the last valid backup.
	if err := validateNoDuplicateJSONKeys(data); err != nil {
		return userProfile{}, fmt.Errorf("некорректные ключи профиля: %w", err)
	}
	var profile userProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return userProfile{}, err
	}
	// The writer retains at most 64 unknown fields. Reject a primary that
	// would silently lose fields during sanitization and hide a valid backup.
	if len(profile.Extra) > 64 {
		return userProfile{}, errors.New("слишком много дополнительных полей профиля")
	}
	// The writer cannot persist extension values larger than 64 KiB.
	// Reject such a primary instead of silently dropping its metadata and
	// hiding the last readable backup during profile sanitization.
	for key, value := range profile.Extra {
		if len([]rune(key)) > 80 {
			return userProfile{}, fmt.Errorf("слишком длинное имя дополнительного поля профиля %q", key)
		}
		if len(value) > 64<<10 {
			return userProfile{}, fmt.Errorf("слишком большое дополнительное поле профиля %q", key)
		}
	}
	// The writer always persists all three preferences together. A partial
	// on-disk settings object must not mask a complete backup; older profiles
	// without a settings object are still supported.
	if err := validateDiskSettingsCompleteness(data); err != nil {
		return userProfile{}, err
	}
	if profile.SchemaVersion <= 0 || profile.SchemaVersion > profileSchemaVersion {
		return userProfile{}, errors.New("неподдерживаемая версия профиля")
	}
	return profile, nil
}

func validateDiskSettingsCompleteness(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key, raw := range fields {
		if !strings.EqualFold(key, "settings") {
			continue
		}
		var settings map[string]json.RawMessage
		if err := json.Unmarshal(raw, &settings); err != nil {
			return err
		}
		// Unknown nested settings cannot survive a rewrite through
		// userSettings. Reject a lossy primary so its backup can be used.
		for name := range settings {
			if !strings.EqualFold(name, "server") && !strings.EqualFold(name, "theme") && !strings.EqualFold(name, "view") {
				return fmt.Errorf("неизвестное поле настроек профиля %q", name)
			}
		}
		for _, required := range [...]string{"server", "theme", "view"} {
			found := false
			for name := range settings {
				if strings.EqualFold(name, required) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("неполные настройки профиля: отсутствует %s", required)
			}
		}
		// The disk writer stores only these canonical preference values.
		// Invalid values in a primary must not conceal a valid backup.
		var decoded userSettings
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return err
		}
		if decoded.Server != "kiss" && decoded.Server != "original" {
			return errors.New("некорректный сервер в профиле")
		}
		if decoded.Theme != "dark" && decoded.Theme != "light" {
			return errors.New("некорректная тема в профиле")
		}
		if decoded.View != "cards" && decoded.View != "list" {
			return errors.New("некорректный режим отображения в профиле")
		}
	}
	return nil
}

func readLimitedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("файл превышает допустимый размер %d байт", maximum)
	}
	return data, nil
}

func sanitizeProfile(profile userProfile) userProfile {
	profile.SchemaVersion = profileSchemaVersion
	if profile.Settings.Server != "kiss" && profile.Settings.Server != "original" {
		profile.Settings.Server = "kiss"
	}
	if profile.Settings.Theme != "dark" && profile.Settings.Theme != "light" {
		profile.Settings.Theme = "dark"
	}
	if profile.Settings.View != "cards" && profile.Settings.View != "list" {
		profile.Settings.View = "cards"
	}

	profile.ItemFilters = map[string]string{}
	profile.MonsterFilters = map[string]string{}
	profile.Favorites = sanitizeStringList(profile.Favorites, 5000, 80, func(value string) bool {
		parts := strings.Split(value, ":")
		if len(parts) != 2 || (parts[0] != "item" && parts[0] != "monster" && parts[0] != "title" && parts[0] != "transformation") {
			return false
		}
		if parts[1] == "" || len(parts[1]) > 20 {
			return false
		}
		for _, r := range parts[1] {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	})
	profile.History = sanitizeStringList(profile.History, 50, 120, func(value string) bool { return value != "" })
	profile.RecentlyViewed = sanitizeRecentViews(profile.RecentlyViewed, 8)
	profile.Extra = sanitizeProfileExtra(profile.Extra)
	profile.UpdatedAt = time.Now().UTC()
	return profile
}

func sanitizeRecentViews(values []recentViewEntry, maxCount int) []recentViewEntry {
	result := make([]recentViewEntry, 0, min(len(values), maxCount))
	seen := make(map[string]struct{}, len(values))
	for _, entry := range values {
		entry.Type = strings.TrimSpace(entry.Type)
		entry.Name = strings.TrimSpace(entry.Name)
		entry.Meta = strings.TrimSpace(entry.Meta)
		if (entry.Type != "item" && entry.Type != "recipe" && entry.Type != "monster" && entry.Type != "title" && entry.Type != "transformation") || entry.ID <= 0 || entry.Name == "" || len([]rune(entry.Name)) > 160 {
			continue
		}
		if len([]rune(entry.Meta)) > 240 {
			entry.Meta = string([]rune(entry.Meta)[:240])
		}
		entry.Server = normalizeServerDataKey(entry.Server)
		if entry.Type == "monster" && entry.Server != "" && entry.Server != "kiss" && entry.Server != "original" {
			continue
		}
		if entry.Type != "monster" {
			entry.Server = ""
		}
		key := fmt.Sprintf("%s:%d:%s", entry.Type, entry.ID, entry.Server)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, entry)
		if len(result) == maxCount {
			break
		}
	}
	return result
}

func sanitizeProfileExtra(values map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage)
	if len(values) == 0 {
		return result
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if len(result) >= 64 {
			break
		}
		if len([]rune(key)) > 80 {
			continue
		}
		if _, known := canonicalProfileKey(key); known {
			continue
		}
		value := values[key]
		if len(value) > 64<<10 || !json.Valid(value) {
			continue
		}
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func copyRawMessageMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func sanitizeStringList(values []string, maxCount, maxLength int, valid func(string) bool) []string {
	result := make([]string, 0, min(len(values), maxCount))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len([]rune(value)) > maxLength || !valid(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		if len(result) == maxCount {
			break
		}
	}
	return result
}

func (p *profileStore) Get() userProfile {
	p.mu.RLock()
	defer p.mu.RUnlock()
	copyProfile := p.profile
	copyProfile.ItemFilters = copyStringMap(p.profile.ItemFilters)
	copyProfile.MonsterFilters = copyStringMap(p.profile.MonsterFilters)
	copyProfile.Favorites = append([]string{}, p.profile.Favorites...)
	copyProfile.History = append([]string{}, p.profile.History...)
	copyProfile.RecentlyViewed = append([]recentViewEntry{}, p.profile.RecentlyViewed...)
	copyProfile.Extra = copyRawMessageMap(p.profile.Extra)
	return copyProfile
}

func copyStringMap(values map[string]string) map[string]string {
	copyValues := make(map[string]string, len(values))
	for key, value := range values {
		copyValues[key] = value
	}
	return copyValues
}

func (p *profileStore) Replace(profile userProfile) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if profile.RecentlyViewed == nil {
		profile.RecentlyViewed = append([]recentViewEntry{}, p.profile.RecentlyViewed...)
	}
	if profile.Extra == nil {
		profile.Extra = copyRawMessageMap(p.profile.Extra)
	} else {
		// Merge preserved metadata into an owned copy. The caller may reuse
		// its map even when the disk write fails.
		profile.Extra = copyRawMessageMap(profile.Extra)
		for key, value := range p.profile.Extra {
			if _, exists := profile.Extra[key]; !exists {
				profile.Extra[key] = append(json.RawMessage(nil), value...)
			}
		}
	}
	profile = sanitizeProfile(profile)
	if err := atomicWriteJSON(p.path, p.backup, profile); err != nil {
		return err
	}
	p.profile = profile
	return nil
}

func (p *profileStore) Flush() error {
	p.mu.RLock()
	profile := p.profile
	profile.ItemFilters = copyStringMap(p.profile.ItemFilters)
	profile.MonsterFilters = copyStringMap(p.profile.MonsterFilters)
	profile.Favorites = append([]string{}, p.profile.Favorites...)
	profile.History = append([]string{}, p.profile.History...)
	profile.RecentlyViewed = append([]recentViewEntry{}, p.profile.RecentlyViewed...)
	profile.Extra = copyRawMessageMap(p.profile.Extra)
	p.mu.RUnlock()
	return atomicWriteJSON(p.path, p.backup, profile)
}

func atomicWriteJSON(path, backup string, value any) error {
	if !safeOwnedFilePath(path) || !safeOwnedFilePath(backup) {
		return errUnsafeDestructivePath
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	// Persist only payloads that their respective disk loaders can read.
	// Profile extensions and large maintenance queues can exceed the limits.
	switch value.(type) {
	case userProfile, *userProfile:
		if int64(len(data)) > maxProfileBytes {
			return fmt.Errorf("профиль превышает допустимый размер %d байт", maxProfileBytes)
		}
	case pendingDeleteList, *pendingDeleteList:
		if int64(len(data)) > maxPendingListBytes {
			return fmt.Errorf("очередь отложенной очистки превышает допустимый размер %d байт", maxPendingListBytes)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".profile-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	defer cleanup()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	old := path + ".old"
	_ = os.Remove(old)
	if _, err := os.Stat(path); err == nil {
		// Profile writes must not replace a valid backup with a corrupt
		// primary. This writer also stores maintenance metadata, which is
		// not a profile and must retain its normal backup behavior.
		backupPrevious := true
		switch value.(type) {
		case userProfile, *userProfile:
			_, err := loadProfileFile(path)
			backupPrevious = err == nil
		case pendingDeleteList, *pendingDeleteList:
			// Do not overwrite the last valid maintenance queue with a
			// damaged primary when recovering from an interrupted write.
			_, err := loadPendingDeleteList(path)
			backupPrevious = err == nil
		}
		if backupPrevious {
			if err := atomicCopyFile(path, backup, 0o600); err != nil {
				return err
			}
		}
		if err := os.Rename(path, old); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		// A missing maintenance queue means the previous generation ended.
		// Its backup must not become the fallback for a newly created queue:
		// replaying it could delete files created at reused paths.
		switch value.(type) {
		case pendingDeleteList, *pendingDeleteList:
			if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	} else {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Rename(old, path)
		return err
	}
	_ = os.Remove(old)
	return syncParentDir(filepath.Dir(path))
}

func atomicCopyFile(source, destination string, mode fs.FileMode) error {
	if !safeOwnedFilePath(source) || !safeOwnedFilePath(destination) {
		return errUnsafeDestructivePath
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".backup-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, in)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if info, err := os.Lstat(destination); err == nil {
		if fileInfoIsReparse(info) || info.IsDir() {
			return errUnsafeDestructivePath
		}
		if err := os.Remove(destination); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return err
	}
	return syncParentDir(filepath.Dir(destination))
}

func syncParentDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	_ = f.Sync()
	return nil
}

type pendingDeleteList struct {
	Paths []string `json:"paths"`
}

func loadPendingDeleteList(path string) (pendingDeleteList, error) {
	if !safeOwnedFilePath(path) {
		return pendingDeleteList{}, errUnsafeDestructivePath
	}
	data, err := readLimitedFile(path, maxPendingListBytes)
	if err != nil {
		return pendingDeleteList{}, err
	}
	if err := validateNoDuplicateJSONKeys(data); err != nil {
		return pendingDeleteList{}, err
	}
	// A valid JSON document is not necessarily a valid maintenance queue:
	// null, {}, and {"paths":null} would silently discard queued cleanup
	// and prevent recovery from an intact backup. Preserve legacy casing,
	// but reject ambiguous case-insensitive aliases.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return pendingDeleteList{}, err
	}
	var rawPaths json.RawMessage
	for key, value := range fields {
		// A future queue format may add safety constraints. Never execute
		// paths while silently ignoring unknown envelope fields.
		if !strings.EqualFold(key, "paths") {
			return pendingDeleteList{}, fmt.Errorf("unknown pending-delete field %q", key)
		}
		if rawPaths != nil {
			return pendingDeleteList{}, errors.New("conflicting pending-delete paths fields")
		}
		rawPaths = value
	}
	array := bytes.TrimSpace(rawPaths)
	if len(array) == 0 || array[0] != '[' {
		return pendingDeleteList{}, errors.New("pending-delete paths must be an array")
	}
	var list pendingDeleteList
	if err := json.Unmarshal(array, &list.Paths); err != nil {
		return pendingDeleteList{}, err
	}
	// JSON null decodes into an empty string in []string. An empty entry
	// makes this primary unusable: accepting it would hide a valid backup
	// and silently discard pending deletions.
	for _, path := range list.Paths {
		if strings.TrimSpace(path) == "" {
			return pendingDeleteList{}, errors.New("pending-delete paths must not contain blank entries")
		}
		// appendPendingDeletes stores absolute paths only. A relative entry
		// cannot be safely replayed and must not hide a valid backup queue.
		if !filepath.IsAbs(path) {
			return pendingDeleteList{}, errors.New("pending-delete paths must be absolute")
		}
		// The maintenance writer persists cleaned paths. A non-canonical
		// primary may mask the last valid backup even though its entries
		// cannot be safely replayed as written.
		if filepath.Clean(path) != path {
			return pendingDeleteList{}, errors.New("pending-delete paths must be canonical")
		}
		// JSON can encode NUL, but no filesystem path can contain it.
		// Reject this unusable primary so a valid backup remains recoverable.
		if strings.IndexByte(path, 0) >= 0 {
			return pendingDeleteList{}, errors.New("pending-delete paths must not contain NUL")
		}
	}
	return list, nil
}

func runMaintenance(paths appPaths, currentExecutable string, logger interface{ Printf(string, ...any) }) {
	allowed := []string{paths.Cache, paths.Temp, paths.Logs, paths.Updates, paths.Versions}
	processPendingDeletes(paths.PendingDelete, allowed, currentExecutable, logger)
	failed := make([]string, 0, 16)
	failed = append(failed, cleanupDirectory(paths.Temp, tempMaxAge, 0, currentExecutable, logger)...)
	failed = append(failed, cleanupDirectory(paths.Cache, cacheMaxAge, cacheMaxBytes, currentExecutable, logger)...)
	failed = append(failed, cleanupDirectory(paths.Updates, updateMaxAge, 0, currentExecutable, logger)...)
	failed = append(failed, cleanupDirectory(paths.Logs, logMaxAge, 0, currentExecutable, logger)...)
	failed = append(failed, cleanupVersions(paths.Versions, 2, currentExecutable, logger)...)
	appendPendingDeletes(paths.PendingDelete, failed, allowed, currentExecutable, logger)
}

func processPendingDeletes(listPath string, allowedRoots []string, currentExecutable string, logger interface{ Printf(string, ...any) }) {
	if !safeOwnedFilePath(listPath) {
		return
	}
	list, err := loadPendingDeleteList(listPath)
	if err != nil {
		// A missing primary normally means cleanup completed. Replaying
		// an old backup could delete newly created files at reused paths.
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		list, err = loadPendingDeleteList(listPath + ".bak")
		if err != nil {
			return
		}
	}
	remaining := make([]string, 0, len(list.Paths))
	for _, candidate := range list.Paths {
		path, root, ok := destructiveRootForPath(candidate, allowedRoots, currentExecutable)
		if !ok {
			continue
		}
		if err := safeRemoveAll(path, root, currentExecutable); err != nil {
			remaining = append(remaining, path)
		}
	}
	if len(remaining) == 0 {
		_ = os.Remove(listPath)
		return
	}
	_ = atomicWriteJSON(listPath, listPath+".bak", pendingDeleteList{Paths: remaining})
	logger.Printf("отложенная очистка: осталось %d путей", len(remaining))
}

func appendPendingDeletes(listPath string, paths []string, allowedRoots []string, currentExecutable string, logger interface{ Printf(string, ...any) }) {
	if len(paths) == 0 || !safeOwnedFilePath(listPath) {
		return
	}
	combined := make([]string, 0, len(paths)+8)
	existing, err := loadPendingDeleteList(listPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		existing, err = loadPendingDeleteList(listPath + ".bak")
	}
	if err == nil {
		combined = append(combined, existing.Paths...)
	}
	combined = append(combined, paths...)
	seen := make(map[string]struct{}, len(combined))
	filtered := make([]string, 0, len(combined))
	for _, candidate := range combined {
		absolute, _, ok := destructiveRootForPath(candidate, allowedRoots, currentExecutable)
		if !ok {
			continue
		}
		if _, exists := seen[absolute]; exists {
			continue
		}
		seen[absolute] = struct{}{}
		filtered = append(filtered, absolute)
	}
	if len(filtered) == 0 {
		return
	}
	if err := atomicWriteJSON(listPath, listPath+".bak", pendingDeleteList{Paths: filtered}); err != nil {
		logger.Printf("не удалось сохранить список отложенной очистки: %v", err)
		return
	}
	logger.Printf("в отложенную очистку добавлено %d путей", len(filtered))
}

func cleanupDirectory(root string, maxAge time.Duration, maxBytes int64, currentExecutable string, logger interface{ Printf(string, ...any) }) []string {
	rootAbs, ok := safeMaintenanceRoot(root)
	if !ok {
		logger.Printf("очистка %s пропущена: небезопасный путь", filepath.Base(root))
		return nil
	}
	now := time.Now()
	type entry struct {
		path    string
		modTime time.Time
		size    int64
	}
	entries := make([]entry, 0, 64)
	failed := make([]string, 0, 8)
	var total int64
	_ = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == rootAbs {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil
		}
		if fileInfoIsReparse(info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if sameFilePath(path, currentExecutable) || !insideRoot(path, rootAbs) {
			return nil
		}
		entries = append(entries, entry{path: path, modTime: info.ModTime(), size: info.Size()})
		total += info.Size()
		return nil
	})
	removed := make(map[string]struct{}, len(entries))
	for _, item := range entries {
		if maxAge > 0 && now.Sub(item.modTime) > maxAge {
			if err := safeRemove(item.path, rootAbs, currentExecutable); err == nil {
				total -= item.size
				removed[item.path] = struct{}{}
			} else {
				failed = append(failed, item.path)
			}
		}
	}
	if maxBytes > 0 && total > maxBytes {
		sort.Slice(entries, func(i, j int) bool { return entries[i].modTime.Before(entries[j].modTime) })
		for _, item := range entries {
			if total <= maxBytes {
				break
			}
			if _, alreadyRemoved := removed[item.path]; alreadyRemoved {
				continue
			}
			if _, err := os.Stat(item.path); err != nil {
				continue
			}
			if err := safeRemove(item.path, rootAbs, currentExecutable); err == nil {
				total -= item.size
			} else {
				failed = append(failed, item.path)
			}
		}
	}
	removeEmptyDirectories(rootAbs, currentExecutable)
	if total < 0 {
		total = 0
	}
	logger.Printf("очистка %s завершена, размер %d байт", filepath.Base(rootAbs), total)
	return failed
}

func cleanupVersions(root string, keep int, currentExecutable string, logger interface{ Printf(string, ...any) }) []string {
	rootAbs, ok := safeMaintenanceRoot(root)
	if !ok {
		logger.Printf("очистка каталога Versions пропущена: небезопасный путь")
		return nil
	}
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		return nil
	}
	type versionEntry struct {
		path string
		mod  time.Time
	}
	versions := make([]versionEntry, 0, len(entries))
	failed := make([]string, 0, 4)
	for _, entry := range entries {
		path := filepath.Join(rootAbs, entry.Name())
		info, err := os.Lstat(path)
		// Retention applies to version directories, not unrelated files.
		// Counting loose files could evict valid rollback versions.
		if err != nil || fileInfoIsReparse(info) || !info.IsDir() {
			continue
		}
		versions = append(versions, versionEntry{path: path, mod: info.ModTime()})
	}
	sort.Slice(versions, func(i, j int) bool {
		if !versions[i].mod.Equal(versions[j].mod) {
			return versions[i].mod.After(versions[j].mod)
		}
		// Timestamp resolution can make successive rollback directories
		// indistinguishable. Prefer the higher version instead of evicting
		// it according to ReadDir's lexical enumeration order.
		left, right := filepath.Base(versions[i].path), filepath.Base(versions[j].path)
		if cmp := compareVersions(left, right); cmp != 0 {
			return cmp > 0
		}
		return left > right
	})
	for _, version := range versions[min(keep, len(versions)):] {
		if err := safeRemoveAll(version.path, rootAbs, currentExecutable); err != nil {
			failed = append(failed, version.path)
		}
	}
	logger.Printf("каталог версий: сохранено не более %d предыдущих версий", keep)
	return failed
}

func removeEmptyDirectories(root, currentExecutable string) {
	dirs := make([]string, 0, 32)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return nil
		}
		if fileInfoIsReparse(info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	})
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		_ = safeRemove(dir, root, currentExecutable)
	}
}

func insideRoot(path, root string) bool {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && rel != "." && rel != "" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}
