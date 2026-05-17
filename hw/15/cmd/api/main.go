// Учебный сервис поиска: GET /search?q=... → проксирует в upstream.
//
// ENV:
//   API_ADDR      — адрес сервиса (по умолчанию :8080)
//   UPSTREAM_URL  — URL поискового бэкенда (по умолчанию http://localhost:9090)
package main

import (
	"cmp"
	"log"
	"net/http"
	"os"
	"time"

	"cachesvc/internal/api"
	"cachesvc/internal/search"
)

func main() {
	addr := cmp.Or(os.Getenv("API_ADDR"), ":8080")
	upstreamURL := cmp.Or(os.Getenv("UPSTREAM_URL"), "http://localhost:9090")

	h := &api.Handler{
		Search: search.NewClient(upstreamURL),
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api listening on %s (upstream=%s)", addr, upstreamURL)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
