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
	"time"
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

		// If cancellation precedes admission, both select cases would be
		// ready while a slot is free. Do not randomly execute a dead request.
		if r.Context().Err() != nil {
			http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
			return
		}

		select {
		case slots <- struct{}{}:
			defer func() {
				<-slots
			}()
			// Cancellation can also race with acquiring the slot.
			if r.Context().Err() != nil {
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			}

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
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// The application context can already be canceled even before the
			// closing flag is visible. Reject every API route, including
			// uncached profile mutations, before it reaches its handler.
			if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
			// Cached API hits bypass the backend concurrency limiter. Reject
			// canceled requests here before they can replay a cached response.
			if r.Context().Err() != nil {
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Apply browser protections even when host/origin validation rejects
		// a request. In particular, forbidden responses must be no-store.
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

	// Both callers require a JSON object. encoding/json accepts a top-level
	// null into a struct without error, which could turn a profile update into
	// a zero-value replacement and erase saved preferences.
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		http.Error(w, "Некорректные данные запроса.\n", http.StatusBadRequest)
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
	committed    http.Header
	status       int
	body         bytes.Buffer
	wroteHeader  bool
	destination  http.ResponseWriter
	maxBodyBytes int64
	streaming    bool
	beforeStream func() error
	duringStream func() error
	aborted      error
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

// Match net/http.ResponseWriter: header mutations after WriteHeader do not
// change the response already committed to the client. Until WriteHeader,
// an endpoint that writes no body may still set its response headers.
func (w *captureWriter) responseHeaders() http.Header {
	if w.wroteHeader {
		return w.committed
	}
	return w.header
}

// forwardCapturedHeaders keeps explicit endpoint cache policy identical on
// buffered and streamed responses. The outer security middleware sets a
// default Cache-Control: no-store; appending the endpoint's explicit policy
// would create conflicting fields on a miss but not on a cache hit. Other
// headers are appended to retain the security middleware's protections.
func forwardCapturedHeaders(destination, captured http.Header) {
	for name, values := range captured {
		if len(values) == 0 {
			continue
		}
		if strings.EqualFold(name, "Cache-Control") {
			destination.Del("Cache-Control")
		}
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func (w *captureWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}

	w.status = status
	w.committed = w.header.Clone()
	w.wroteHeader = true
}

func (w *captureWriter) Write(data []byte) (int, error) {
	if w.aborted != nil {
		return 0, w.aborted
	}
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.destination != nil && !w.streaming &&
		int64(len(data)) > w.maxBodyBytes-int64(w.body.Len()) {
		// Streaming commits directly to the client, bypassing the buffered
		// response's post-backend lifecycle check. Reject a canceled request
		// before committing any of its pending headers or body bytes.
		if w.beforeStream != nil {
			if err := w.beforeStream(); err != nil {
				w.aborted = err
				w.body.Reset()
				return 0, err
			}
		}
		forwardCapturedHeaders(w.destination.Header(), w.responseHeaders())
		w.destination.WriteHeader(w.status)
		w.streaming = true
		if w.body.Len() > 0 {
			// The destination may report a short write without an error.
			// Never silently discard the missing prefix and append later data.
			n, err := w.destination.Write(w.body.Bytes())
			if err == nil && n != w.body.Len() {
				err = io.ErrShortWrite
			}
			w.body.Reset()
			if err != nil {
				w.aborted = err
				return 0, err
			}
		}
	}
	if w.streaming {
		// After headers have been committed, an HTTP error response is no
		// longer possible. Stop writing further body chunks on cancellation
		// or shutdown without appending an error page to the partial stream.
		if w.duringStream != nil {
			if err := w.duringStream(); err != nil {
				w.aborted = err
				return 0, err
			}
		}
		n, err := w.destination.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		if err != nil {
			// Once the stream is committed, a failed write cannot be
			// repaired. Reject subsequent writes instead of appending a
			// misleading continuation to the truncated representation.
			w.aborted = err
		}
		return n, err
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
			// A lookup or singleflight wake-up can outlive the initial lifecycle
			// check. Never replay a cached body after cancellation or shutdown.
			if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
			if r.Context().Err() != nil {
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			}
			for name, value := range entry.header {
				w.Header().Set(name, value)
			}
			// A cache hit must not reset the client's max-age freshness clock.
			// Age is generated at replay time, not copied from the origin.
			elapsed := entry.ttl - time.Until(entry.expiresAt)
			if elapsed < 0 {
				elapsed = 0
			}
			w.Header().Set("Age", strconv.FormatInt(int64(elapsed/time.Second), 10))
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
			if ready == nil {
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
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
				// Request cancellation and application shutdown may both
				// become ready while a follower waits. Preserve the 503
				// shutdown priority regardless of which select arm wins.
				if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
					http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
					return
				}
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return
			case <-appDone:
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return
			}
		}

		captured := newStreamingCaptureWriter(w, a.cache.maxEntryBytes)
		captured.duringStream = func() error {
			if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
				return context.Canceled
			}
			return r.Context().Err()
		}
		captured.beforeStream = func() error {
			if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
				http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
				return context.Canceled
			}
			if err := r.Context().Err(); err != nil {
				http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
				return err
			}
			return nil
		}
		next.ServeHTTP(captured, r)
		if captured.streaming || captured.aborted != nil {
			return
		}
		// The backend may finish after the request was canceled or shutdown
		// started. A buffered response has not been committed yet: discard it
		// rather than publishing its body or storing it in the cache.
		if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) {
			http.Error(w, "Приложение завершает работу.\n", http.StatusServiceUnavailable)
			return
		}
		if r.Context().Err() != nil {
			http.Error(w, "Запрос отменён.\n", http.StatusRequestTimeout)
			return
		}

		body := captured.body.Bytes()

		forwardCapturedHeaders(w.Header(), captured.responseHeaders())

		w.WriteHeader(captured.status)
		written, writeErr := w.Write(body)
		// A failed or short destination write cannot be treated as a
		// successfully delivered representation. The status is already
		// committed, but the incomplete response must not seed the cache.
		if writeErr != nil || written != len(body) {
			return
		}

		// Cancellation or shutdown may begin while the destination commits
		// the buffered body. The response is already visible and cannot be
		// replaced with an error, but it must not populate the shared cache.
		if a.closing.Load() || (a.ctx != nil && a.ctx.Err() != nil) || r.Context().Err() != nil {
			return
		}

		headers, ok := cacheableResponseHeaders(captured.responseHeaders())
		if !ok {
			return
		}

		lifetime, ok := responseCacheLifetime(captured.responseHeaders(), a.cache.ttl)
		if !ok {
			return
		}
		a.cache.PutWithTTL(key, captured.status, headers, body, lifetime)
	})
}

// responseCacheLifetime caps internal freshness to both max-age and s-maxage.
// This in-process cache is shared across HTTP requests, so s-maxage=0 must
// disable reuse even when max-age is positive. Invalid or duplicate freshness
// directives fail closed rather than silently extending the cached lifetime.
// The caller has already rejected multi-valued Cache-Control headers.
func responseCacheLifetime(header http.Header, configured time.Duration) (time.Duration, bool) {
	value := header.Get("Cache-Control")
	if value == "" {
		return configured, configured > 0
	}
	seenMaxAge, seenSharedMaxAge := false, false
	lifetime := configured
	for _, directive := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(directive), "=", 2)
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		if name != "max-age" && name != "s-maxage" {
			continue
		}
		if len(parts) != 2 || (name == "max-age" && seenMaxAge) || (name == "s-maxage" && seenSharedMaxAge) {
			return 0, false
		}
		if name == "max-age" {
			seenMaxAge = true
		} else {
			seenSharedMaxAge = true
		}
		age := strings.TrimSpace(parts[1])
		if len(age) >= 2 && age[0] == '"' && age[len(age)-1] == '"' {
			age = age[1 : len(age)-1]
		}
		if age == "" {
			return 0, false
		}
		for _, digit := range age {
			if digit < '0' || digit > '9' {
				return 0, false
			}
		}
		seconds, err := strconv.ParseUint(age, 10, 64)
		if err != nil || seconds == 0 {
			return 0, false
		}
		// Compare before multiplying to avoid time.Duration overflow.
		if seconds <= uint64(lifetime/time.Second) {
			lifetime = time.Duration(seconds) * time.Second
		}
	}
	return lifetime, lifetime > 0
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
