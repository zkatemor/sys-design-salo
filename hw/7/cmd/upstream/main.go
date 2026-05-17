// Flaky upstream.
//
//   GET  /data            — режим healthy/down/slow
//   POST /admin/healthy   — переключить в healthy
//   POST /admin/down      — переключить в down (всегда 500)
//   POST /admin/slow      — переключить в slow (5s, потом 200)
//   GET  /admin/mode      — текущий режим
//
// Адрес: $UPSTREAM_ADDR (по умолчанию :9090).
package main

import (
	"cmp"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

const (
	modeHealthy = "healthy"
	modeDown    = "down"
	modeSlow    = "slow"
)

func main() {
	addr := cmp.Or(os.Getenv("UPSTREAM_ADDR"), ":9090")

	var mode atomic.Value
	mode.Store(modeHealthy)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /data", func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load().(string) {
		case modeDown:
			http.Error(w, "upstream is down", http.StatusInternalServerError)
		case modeSlow:
			select {
			case <-time.After(5 * time.Second):
				fmt.Fprintln(w, `{"data":"ok-but-slow"}`)
			case <-r.Context().Done():
			}
		default:
			fmt.Fprintln(w, `{"data":"ok"}`)
		}
	})

	setMode := func(m string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			mode.Store(m)
			fmt.Fprintf(w, "mode=%s\n", m)
		}
	}
	mux.HandleFunc("POST /admin/healthy", setMode(modeHealthy))
	mux.HandleFunc("POST /admin/down", setMode(modeDown))
	mux.HandleFunc("POST /admin/slow", setMode(modeSlow))
	mux.HandleFunc("GET /admin/mode", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "mode=%s\n", mode.Load().(string))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("upstream listening on %s (initial mode=%s)", addr, modeHealthy)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
