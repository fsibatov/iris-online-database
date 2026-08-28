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
	"strings"
)

const applicationID = "iris-online-database"

var appVersion = "2.0.4"

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
	writeJSON(w, a.updates.Check(ctx, force))
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

	var handler http.Handler = mux
	handler = a.withResponseCache(handler)
	handler = a.withLifecycleGuard(handler)
	handler = withAPIConcurrencyLimit(handler, 32)
	handler = withSecurityHeaders(handler)

	return handler
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
	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{
		header: make(http.Header),
		status: http.StatusOK,
	}
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

	return w.body.Write(data)
}

func isCacheableResponseRequest(r *http.Request) bool {
	if r == nil || r.Method != http.MethodGet {
		return false
	}

	path := r.URL.Path

	return path == "/api/meta" ||
		path == "/api/search" ||
		path == "/api/items" ||
		strings.HasPrefix(path, "/api/items/") ||
		path == "/api/recipes" ||
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

		key := r.URL.RequestURI()

		if entry, ok := a.cache.Get(key); ok {
			for name, value := range entry.header {
				w.Header().Set(
					name,
					value,
				)
			}

			w.WriteHeader(entry.status)
			_, _ = w.Write(entry.body)
			return
		}

		captured := newCaptureWriter()
		next.ServeHTTP(captured, r)

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

		headers := map[string]string{}

		for _, name := range []string{
			"Content-Type",
		} {
			if value := captured.header.Get(name); value != "" {
				headers[name] = value
			}
		}

		a.cache.Put(
			key,
			captured.status,
			headers,
			body,
		)
	})
}
