package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const applicationID = "iris-online-database"

var appVersion = "2.0.5"

//go:embed web/* data/latest-vk.json assets/game_data.json.gz assets/set_effects.json.gz assets/item_abilities.json.gz assets/item_recipes.json.gz assets/quest_reward_sources.json.gz assets/monster_details.json.gz assets/chest_contents.json.gz assets/monster_presence.json.gz assets/transformation_cards.json.gz assets/item_enhancements.json.gz
var embedded embed.FS

func (a *application) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	ctx := r.Context()
	if a.ctx != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		stop := context.AfterFunc(a.ctx, cancel)
		defer func() {
			stop()
			cancel()
		}()
	}

	force := r.URL.Query().Get("refresh") == "1"
	result := a.updates.Check(ctx, force)
	if result.Failure != "" && result.Failure != updateFailureCanceled && a.logger != nil {
		if result.diagnostic != "" {
			a.logger.Printf("проверка обновлений: %s: %s", result.Failure, result.diagnostic)
		} else {
			a.logger.Printf("проверка обновлений: %s", result.Failure)
		}
	}
	writeJSON(w, result)
}

func (a *application) handleCommunityStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	ctx := r.Context()
	if a.ctx != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		stop := context.AfterFunc(a.ctx, cancel)
		defer func() {
			stop()
			cancel()
		}()
	}

	force := r.URL.Query().Get("refresh") == "1"
	writeJSON(w, a.community.Check(ctx, force))
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/update-check", a.handleUpdateCheck)
	mux.HandleFunc("/api/community-status", a.handleCommunityStatus)
	mux.HandleFunc("/api/user-data", a.handleUserData)
	mux.HandleFunc("/api/favorites", handleFavorites)
	mux.HandleFunc("/api/meta", handleMeta)
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/items", handleItems)
	mux.HandleFunc("/api/items/", handleItem)
	mux.HandleFunc("/api/recipes", handleRecipes)
	mux.HandleFunc("/api/titles", handleTitles)
	mux.HandleFunc("/api/titles/", handleTitle)
	mux.HandleFunc("/api/transformations", handleTransformations)
	mux.HandleFunc("/api/transformations/", handleTransformation)
	mux.HandleFunc("/api/world-source-monsters", handleWorldSourceMonsters)
	mux.HandleFunc("/api/monster-world-drops", handleMonsterWorldDrops)
	mux.HandleFunc("/api/monsters", handleMonsters)
	mux.HandleFunc("/api/monsters/", handleMonster)

	webFS, err := fs.Sub(embedded, "web")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(webFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(
				w,
				http.MethodGet,
				http.MethodHead,
			)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(webFS, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	return a.withAPIRequestPipeline(mux)
}

// Cache followers wait outside the backend limiter, so repeated requests do
// not exhaust its 32 execution slots. Security and shutdown guards run first.
func (a *application) withAPIRequestPipeline(next http.Handler) http.Handler {
	next = withAPIConcurrencyLimit(next, 32)
	next = a.withResponseCache(next)
	next = a.withLifecycleGuard(next)
	return withSecurityHeaders(next)
}

func withAPIConcurrencyLimit(next http.Handler, maximum int) http.Handler {
	if maximum < 1 {
		maximum = 1
	}

	slots := make(chan struct{}, maximum)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		select {
		case slots <- struct{}{}:
			defer func() {
				<-slots
			}()

			next.ServeHTTP(w, r)

		case <-r.Context().Done():
			http.Error(
				w,
				"Запрос отменён.\n",
				http.StatusRequestTimeout,
			)

		default:
			http.Error(
				w,
				"Слишком много одновременных запросов.\n",
				http.StatusServiceUnavailable,
			)
		}
	})
}

func (a *application) withLifecycleGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.closing.Load() && strings.HasPrefix(r.URL.Path, "/api/") {

			http.Error(
				w,
				"Приложение завершает работу.\n",
				http.StatusServiceUnavailable,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validRequestHost(r.Host) {
			http.Error(
				w,
				"Недопустимый адрес запроса.\n",
				http.StatusForbidden,
			)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			if strings.EqualFold(
				r.Header.Get("Sec-Fetch-Site"),
				"cross-site",
			) || !validRequestOrigin(r) {

				http.Error(
					w,
					"Запрос из другого источника отклонён.\n",
					http.StatusForbidden,
				)
				return
			}
		}

		w.Header().Set(
			"Cache-Control",
			"no-store",
		)
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src 'none'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
		)
		w.Header().Set(
			"Cross-Origin-Opener-Policy",
			"same-origin",
		)
		w.Header().Set(
			"Cross-Origin-Resource-Policy",
			"same-origin",
		)
		w.Header().Set(
			"Permissions-Policy",
			"camera=(), microphone=(), geolocation=()",
		)
		w.Header().Set(
			"Referrer-Policy",
			"no-referrer",
		)
		w.Header().Set(
			"X-Content-Type-Options",
			"nosniff",
		)
		w.Header().Set(
			"X-Frame-Options",
			"DENY",
		)

		next.ServeHTTP(w, r)
	})
}

func validRequestHost(hostPort string) bool {
	return strings.EqualFold(strings.TrimSpace(hostPort), "wails.localhost")
}

func validRequestOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(
		r.Header.Get("Origin"),
	)

	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil ||
		parsed.Scheme != "http" ||
		parsed.User != nil ||
		parsed.Path != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return false
	}

	return strings.EqualFold(
		parsed.Host,
		r.Host,
	) && validRequestHost(parsed.Host)
}

func decodeJSONRequest(
	w http.ResponseWriter,
	r *http.Request,
	target any,
	maxBytes int64,
) bool {
	mediaType, _, err := mime.ParseMediaType(
		r.Header.Get("Content-Type"),
	)

	if err != nil || mediaType != "application/json" {
		http.Error(
			w,
			"Требуется Content-Type application/json.\n",
			http.StatusUnsupportedMediaType,
		)
		return false
	}

	data, err := io.ReadAll(
		http.MaxBytesReader(
			w,
			r.Body,
			maxBytes,
		),
	)

	if err != nil {
		var tooLarge *http.MaxBytesError

		if errors.As(err, &tooLarge) {
			http.Error(
				w,
				"Запрос превышает допустимый размер.\n",
				http.StatusRequestEntityTooLarge,
			)
		} else {
			http.Error(
				w,
				"Некорректные данные запроса.\n",
				http.StatusBadRequest,
			)
		}

		return false
	}

	if err := validateNoDuplicateJSONKeys(data); err != nil {
		http.Error(
			w,
			"Некорректные данные запроса.\n",
			http.StatusBadRequest,
		)
		return false
	}

	decoder := json.NewDecoder(
		bytes.NewReader(data),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		http.Error(
			w,
			"Некорректные данные запроса.\n",
			http.StatusBadRequest,
		)
		return false
	}

	var trailing any

	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(
			w,
			"Некорректные данные запроса.\n",
			http.StatusBadRequest,
		)
		return false
	}

	return true
}

func validateNoDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(
		bytes.NewReader(data),
	)

	if err := walkJSONValue(decoder); err != nil {
		return err
	}

	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New(
				"trailing JSON value",
			)
		}
		return err
	}

	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})

		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}

			key, ok := keyToken.(string)
			if !ok {
				return errors.New(
					"invalid JSON object key",
				)
			}

			if _, exists := seen[key]; exists {
				return fmt.Errorf(
					"duplicate JSON field %q",
					key,
				)
			}

			seen[key] = struct{}{}

			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}

		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New(
				"invalid JSON object",
			)
		}

	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}

		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New(
				"invalid JSON array",
			)
		}

	default:
		return errors.New(
			"invalid JSON delimiter",
		)
	}

	return nil
}

func methodNotAllowed(
	w http.ResponseWriter,
	allowed ...string,
) {
	if len(allowed) > 0 {
		w.Header().Set(
			"Allow",
			strings.Join(allowed, ", "),
		)
	}

	http.Error(
		w,
		"Метод запроса не поддерживается.\n",
		http.StatusMethodNotAllowed,
	)
}

type captureWriter struct {
	header       http.Header
	status       int
	body         bytes.Buffer
	wroteHeader  bool
	destination  http.ResponseWriter
	maxBodyBytes int64
	streaming    bool
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

// Oversized responses bypass the cache and stream to the client without
// buffering the complete response in memory.
func newStreamingCaptureWriter(destination http.ResponseWriter, maxBodyBytes int64) *captureWriter {
	writer := newCaptureWriter()
	writer.destination = destination
	writer.maxBodyBytes = maxBodyBytes
	return writer
}

func (w *captureWriter) Header() http.Header {
	return w.header
}

func (w *captureWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}

	w.status = status
	w.wroteHeader = true
}

func (w *captureWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.destination != nil && !w.streaming &&
		int64(len(data)) > w.maxBodyBytes-int64(w.body.Len()) {
		for name, values := range w.header {
			for _, value := range values {
				w.destination.Header().Add(name, value)
			}
		}
		w.destination.WriteHeader(w.status)
		w.streaming = true
		if w.body.Len() > 0 {
			if _, err := w.destination.Write(w.body.Bytes()); err != nil {
				w.body.Reset()
				return 0, err
			}
			w.body.Reset()
		}
	}
	if w.streaming {
		return w.destination.Write(data)
	}
	return w.body.Write(data)
}

func isCacheableResponseRequest(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}

	// Never reuse a response for credentialed, ranged, conditional or
	// explicitly uncached requests. Credentials must not enter the cache key.
	for _, name := range []string{
		"Authorization", "Cookie", "Range", "If-Match", "If-None-Match",
		"If-Modified-Since", "If-Unmodified-Since", "If-Range",
		"Cache-Control", "Pragma",
	} {
		if len(r.Header.Values(name)) != 0 {
			return false
		}
	}

	path := r.URL.Path

	return path == "/api/meta" ||
		path == "/api/search" ||
		path == "/api/items" ||
		strings.HasPrefix(path, "/api/items/") ||
		path == "/api/recipes" ||
		path == "/api/titles" ||
		strings.HasPrefix(path, "/api/titles/") ||
		path == "/api/transformations" ||
		strings.HasPrefix(path, "/api/transformations/") ||
		path == "/api/world-source-monsters" ||
		path == "/api/monster-world-drops" ||
		path == "/api/monsters" ||
		strings.HasPrefix(path, "/api/monsters/")
}

func (a *application) withResponseCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isCacheableResponseRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := cacheKeyForRequest(r)
		if len(key) > maxCacheKeyBytes {
			next.ServeHTTP(w, r)
			return
		}

		writeCached := func(entry cacheEntry) {
			for name, value := range entry.header {
				w.Header().Set(name, value)
			}
			w.WriteHeader(entry.status)
			_, _ = w.Write(entry.body)
		}
		if entry, ok := a.cache.Get(key); ok {
			writeCached(entry)
			return
		}

		var appDone <-chan struct{}
		if a.ctx != nil {
			appDone = a.ctx.Done()
		}
		// A response can be private, oversized or otherwise uncacheable.
		// Retry leadership rather than allowing all waiters to flood the
		// backend after the first response fails to populate the cache.
		for {
			if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
			if r.Context().Err() != nil {
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			}
			ready, release := a.cache.beginLoad(key)
			if release != nil {
				defer release()
				// Recheck after acquiring leadership; shutdown can race.
				if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
					http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
					return
				}
				// Another producer might have populated the cache since Get.
				if entry, ok := a.cache.Get(key); ok {
					writeCached(entry)
					return
				}
				break
			}
			select {
			case <-ready:
				if entry, ok := a.cache.Get(key); ok {
					writeCached(entry)
					return
				}
			case <-r.Context().Done():
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			case <-appDone:
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
		}

		captured := newStreamingCaptureWriter(w, a.cache.maxEntryBytes)
		next.ServeHTTP(captured, r)
		if captured.streaming {
			return
		}

		body := captured.body.Bytes()

		for name, values := range captured.header {
			for _, value := range values {
				w.Header().Add(
					name,
					value,
				)
			}
		}

		w.WriteHeader(captured.status)
		_, _ = w.Write(body)

		headers, ok := cacheableResponseHeaders(captured.header)
		if !ok {
			return
		}

		a.cache.Put(
			key,
			captured.status,
			headers,
			body,
		)
	})
}

// cacheableResponseHeaders retains only metadata that the cache can reproduce
// byte-for-byte. An unknown or repeated field must bypass the cache: silently
// dropping it on replay can change response semantics or privacy guarantees.
func cacheableResponseHeaders(header http.Header) (map[string]string, bool) {
	cached := make(map[string]string, len(header))
	for rawName, values := range header {
		name := http.CanonicalHeaderKey(rawName)
		switch name {
		case "Content-Type", "Content-Language", "Cache-Control", "Etag", "Last-Modified":
		default:
			return nil, false
		}
		if len(values) != 1 || values[0] == "" {
			return nil, false
		}
		if _, exists := cached[name]; exists {
			return nil, false
		}
		if name == "Cache-Control" && hasPrivateCacheDirective(values[0]) {
			return nil, false
		}
		cached[name] = values[0]
	}
	return cached, true
}

// cacheKeyForRequest distinguishes representations without caching credentials.
// Length-prefixed fields avoid delimiter collisions between different headers.
func cacheKeyForRequest(r *http.Request) string {
	var key strings.Builder
	key.WriteString(r.URL.RequestURI())
	for _, name := range []string{"Accept", "Accept-Language", "Accept-Encoding", "Accept-Charset"} {
		values := r.Header.Values(name)
		key.WriteByte(0)
		key.WriteString(name)
		for _, value := range values {
			key.WriteByte(0)
			key.WriteString(strconv.Itoa(len(value)))
			key.WriteByte(':')
			key.WriteString(value)
		}
	}
	return key.String()
}

// hasPrivateCacheDirective implements conservative caching: do not retain
// representations explicitly marked private, non-cacheable or immediately stale.
func hasPrivateCacheDirective(value string) bool {
	for _, directive := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(directive), "=", 2)
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		switch name {
		case "private", "no-store", "no-cache":
			return true
		case "max-age":
			if len(parts) == 2 {
				age := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				if parsed, err := strconv.ParseUint(age, 10, 64); err == nil && parsed == 0 {
					return true
				}
			}
		}
	}
	return false
}
