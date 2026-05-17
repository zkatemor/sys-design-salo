// Учебный сервис заказов.
//
// ENV:
//   API_ADDR                    — адрес сервиса (по умолчанию :8080)
//   OTEL_EXPORTER_OTLP_ENDPOINT — OTLP HTTP endpoint (по умолчанию localhost:4318)
//   OTEL_TRACES_EXPORTER        — stdout | otlp (по умолчанию otlp)
package main

import (
	"cmp"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"obssvc/internal/api"
	"obssvc/internal/observability"
	"obssvc/internal/orders"
	"obssvc/internal/store"
)

func main() {
	addr := cmp.Or(os.Getenv("API_ADDR"), ":8080")

	ctx := context.Background()
	tel, shutdown, err := observability.Setup(ctx, "obssvc")
	if err != nil {
		log.Fatalf("observability setup: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}()

	db := store.New()
	svc := orders.NewService(db, tel)
	h := api.NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("POST /orders", h.CreateOrder)
	mux.HandleFunc("GET /orders/{id}", h.GetOrder)
	mux.Handle("GET /metrics", promhttp.HandlerFor(tel.Reg, promhttp.HandlerOpts{}))

	var handler http.Handler = otelhttp.NewHandler(
		api.Middleware(tel)(mux),
		"http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if p := r.Pattern; p != "" {
				return r.Method + " " + p
			}
			return r.Method + " " + r.URL.Path
		}),
	)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	sigCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Printf("api listening on %s (metrics /metrics, traces OTLP)", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-sigCtx.Done()
	log.Print("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
}
