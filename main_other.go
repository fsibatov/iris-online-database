//go:build !windows

package main

import (
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	app, err := newApplication()
	if err != nil {
		log.Fatal(err)
	}

	handler := app.routes()

	auditHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := r.Clone(r.Context())
		request.Header = r.Header.Clone()
		request.Host = "wails.localhost"

		if strings.HasPrefix(request.URL.Path, "/api/") {
			request.Header.Set("Origin", "http://wails.localhost")
			request.Header.Set("Sec-Fetch-Site", "same-origin")
		}

		handler.ServeHTTP(w, request)
	})

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	log.Printf("Iris Online Database audit server: http://0.0.0.0:%s", port)

	if err := http.ListenAndServe("0.0.0.0:"+port, auditHandler); err != nil {
		log.Fatal(err)
	}
}
