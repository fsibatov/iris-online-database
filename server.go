package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const applicationID = "iris-online-database"

// appVersion is a variable so release and diagnostic builds can pin the visible version
// with -ldflags while development builds keep a safe diagnostic default.
var (
	appVersion    = "1.0.1"
	releaseMarker = "IrisOnlineDiagnostic/1.0.1/development"
)

//go:embed web/* assets/game_data.json.gz assets/set_effects.json.gz assets/item_abilities.json.gz assets/item_recipes.json.gz assets/monster_details.json.gz assets/chest_contents.json.gz
var embedded embed.FS

func main() {
	os.Exit(run())
}

func run() int {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	address := flags.String("addr", "127.0.0.1:8765", "HTTP listen address")
	noBrowser := flags.Bool("no-browser", false, "do not open the browser")
	shutdownWhenIdle := flags.Bool("shutdown-when-idle", false, "stop after the last browser session closes")
	idleGrace := flags.Duration("idle-grace", sessionShutdownGrace, "grace period before idle shutdown")
	heartbeatTimeout := flags.Duration("heartbeat-timeout", sessionHeartbeatTimeout, "maximum interval between browser heartbeats")
	startupTimeout := flags.Duration("startup-timeout", startupSessionTimeout, "stop if the browser interface does not open in time")

	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}

	if *idleGrace <= 0 {
		fmt.Fprintln(os.Stderr, "idle-grace должен быть положительным")
		return 2
	}
	if *heartbeatTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "heartbeat-timeout должен быть положительным")
		return 2
	}
	if *startupTimeout < 0 {
		fmt.Fprintln(os.Stderr, "startup-timeout не может быть отрицательным")
		return 2
	}
	if err := validateListenAddress(*address); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	browserEnabled := !*noBrowser && os.Getenv("IRIS_NO_BROWSER") == ""

	// Headless/test/server operation must never block on a native Windows
	// MessageBox. Interactive launches keep the normal user-facing dialog.
	showStartupError := func(message string) {
		if !browserEnabled {
			fmt.Fprintln(os.Stderr, message)
			return
		}
		showStartupMessage(message)
	}

	// Probe before touching profile/cache/log paths so a second or different
	// Iris Online build never performs maintenance alongside the active copy.
	if existing := probeExistingInstance(*address); existing.Found {
		if sameApplicationBuild(existing) {
			if browserEnabled {
				_ = openBrowser("http://" + *address)
			}
			return 0
		}

		version := strings.TrimSpace(existing.Version)
		if version == "" {
			version = "другой версии"
		}

		showStartupError(
			fmt.Sprintf(
				"Закройте уже запущенную Iris Online %s и повторите запуск.",
				version,
			),
		)
		return 1
	}

	paths, err := resolveAppPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	instanceLock, err := acquireInstanceLock(paths)
	if err != nil {
		if errors.Is(err, errInstanceAlreadyRunning) {
			showStartupError(
				"Iris Online уже запущена. Закройте текущее окно приложения и повторите запуск.",
			)
		} else {
			fmt.Fprintln(
				os.Stderr,
				"блокировка единственного экземпляра:",
				err,
			)
			showStartupError(
				"Не удалось безопасно запустить Iris Online. Проверьте доступ к локальным данным приложения и повторите запуск.",
			)
		}
		return 1
	}
	defer instanceLock.Close()

	executable, _ := os.Executable()
	bootstrapLogger := log.New(
		os.Stderr,
		"",
		log.Ldate|log.Ltime|log.Lmicroseconds,
	)
	runMaintenance(paths, executable, bootstrapLogger)

	logWriter, err := newRotatingLogWriter(
		filepath.Join(paths.Logs, "application.log"),
		2<<20,
		5,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	logger := log.New(
		io.MultiWriter(os.Stderr, logWriter),
		"",
		log.Ldate|log.Ltime|log.Lmicroseconds,
	)

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		existing := probeExistingInstance(*address)

		if sameApplicationBuild(existing) {
			if browserEnabled {
				_ = openBrowser("http://" + *address)
			}

			logger.Printf(
				"вторая копия перенаправлена в уже запущенное приложение той же версии",
			)
			_ = logWriter.Close()
			return 0
		}

		if existing.Found {
			version := strings.TrimSpace(existing.Version)
			if version == "" {
				version = "другой версии"
			}

			message := fmt.Sprintf(
				"Закройте уже запущенную Iris Online %s и повторите запуск.",
				version,
			)

			logger.Printf(
				"несовместимая уже запущенная копия: version=%q release=%q",
				existing.Version,
				existing.Release,
			)

			showStartupError(message)
			_ = logWriter.Close()
			return 1
		}

		logger.Printf(
			"запуск локального сервера: %v",
			err,
		)

		showStartupError(
			"Не удалось запустить локальный сервер Iris Online. Проверьте, не занял ли другой процесс локальный порт приложения.",
		)

		_ = logWriter.Close()
		return 1
	}

	closeListener := true
	defer func() {
		if closeListener {
			_ = listener.Close()
		}
	}()

	profile, err := newProfileStore(paths)
	if err != nil {
		logger.Printf(
			"профиль пользователя: %v",
			err,
		)
		_ = logWriter.Close()
		return 1
	}

	if err := ensureLoaded(); err != nil {
		logger.Printf(
			"загрузка базы данных: %v",
			err,
		)
		_ = logWriter.Close()
		return 1
	}

	closeListener = false

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	ctx, cancel := context.WithCancel(signalContext)

	effectiveStartupTimeout := time.Duration(0)
	if browserEnabled {
		effectiveStartupTimeout = *startupTimeout
	}

	app := &application{
		paths:     paths,
		profile:   profile,
		cache:     newResponseCache(128, 8<<20, 5*time.Minute),
		logger:    logger,
		logWriter: logWriter,
		listener:  listener,
		ctx:       ctx,
		cancel:    cancel,
		sessions:  newSessionManager(*idleGrace, *heartbeatTimeout),
		autoExit:  browserEnabled || *shutdownWhenIdle,

		// Browsers may suspend background tabs for longer than the heartbeat
		// lease. In the normal browser-launched mode, heartbeat expiry therefore
		// means "session needs to reopen", not "the user closed the app".
		// The explicit -shutdown-when-idle mode keeps lease expiry semantics for
		// bounded/headless operation.
		shutdownOnHeartbeatExpiry: *shutdownWhenIdle,
		startupTimeout:            effectiveStartupTimeout,
	}

	app.server = &http.Server{
		Addr:              *address,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	app.wg.Add(1)
	go app.monitorSessions()

	serveError := make(chan error, 1)

	app.wg.Add(1)
	go func() {
		defer app.wg.Done()

		err := app.server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			serveError <- err
		}
		close(serveError)
	}()

	urlText := "http://" + *address

	fmt.Println(urlText)

	logger.Printf(
		"приложение запущено: %s, pid=%d",
		urlText,
		processID(),
	)

	if browserEnabled {
		app.wg.Add(1)

		go func() {
			defer app.wg.Done()

			timer := time.NewTimer(350 * time.Millisecond)
			defer stopTimer(timer)

			select {
			case <-timer.C:
				if err := openBrowser(urlText); err != nil {
					logger.Printf(
						"открытие браузера: %v",
						err,
					)
				}

			case <-ctx.Done():
			}
		}()
	}

	select {
	case <-ctx.Done():
		app.requestShutdown(
			"сигнал завершения или закрытие интерфейса",
		)

	case err := <-serveError:
		if err != nil {
			logger.Printf(
				"ошибка локального сервера: %v",
				err,
			)

			app.requestShutdown(
				"ошибка локального сервера",
			)
		}
	}

	if err := app.shutdown(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func (a *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/session/open", a.handleSessionOpen)
	mux.HandleFunc("/api/session/heartbeat", a.handleSessionHeartbeat)
	mux.HandleFunc("/api/session/close", a.handleSessionClose)
	mux.HandleFunc("/api/user-data", a.handleUserData)
	mux.HandleFunc("/api/favorites", handleFavorites)
	mux.HandleFunc("/api/meta", handleMeta)
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/items", handleItems)
	mux.HandleFunc("/api/items/", handleItem)
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
		if a.closing.Load() &&
			strings.HasPrefix(r.URL.Path, "/api/") &&
			r.URL.Path != "/api/session/close" {

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
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
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
	host := strings.TrimSpace(hostPort)

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else {
		host = strings.Trim(host, "[]")

		if strings.Contains(host, ":") &&
			net.ParseIP(host) == nil {
			return false
		}
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
