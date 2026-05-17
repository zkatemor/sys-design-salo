// «Дорогой» поисковый бэкенд: каждый /search спит 300ms.
//
// Эндпоинты:
//   GET  /search?q=...   — поиск (300ms latency, 5 результатов)
//   GET  /admin/stats    — сколько раз вызывали /search
//   POST /admin/reset    — сбросить счётчик
//
// Адрес: $UPSTREAM_ADDR (по умолчанию :9090).
package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type Result struct {
	Title string `json:"title"`
	Score int    `json:"score"`
}

func main() {
	addr := cmp.Or(os.Getenv("UPSTREAM_ADDR"), ":9090")

	var calls atomic.Int64
	mux := http.NewServeMux()

	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query().Get("q")
		select {
		case <-time.After(300 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResults(q))
	})

	mux.HandleFunc("GET /admin/stats", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "calls=%d\n", calls.Load())
	})
	mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, _ *http.Request) {
		calls.Store(0)
		fmt.Fprintln(w, "reset")
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("upstream listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func mockResults(q string) []Result {
	out := make([]Result, 5)
	for i := range out {
		out[i] = Result{
			Title: fmt.Sprintf("%s result #%d", q, i+1),
			Score: 100 - i*7,
		}
	}
	return out
}
