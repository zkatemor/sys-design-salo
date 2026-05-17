package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"obssvc/internal/observability"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Middleware логирует запросы и пишет HTTP-метрики Prometheus.
func Middleware(tel *observability.Telemetry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tel.HTTPActiveConnections.Inc()
			defer tel.HTTPActiveConnections.Dec()

			start := time.Now()
			path := r.Pattern
			if path == "" {
				path = r.URL.Path
			}

			traceID := trace.SpanFromContext(r.Context()).SpanContext().TraceID().String()
			otel.GetTextMapPropagator().Inject(r.Context(), propagation.HeaderCarrier(w.Header()))

			slog.InfoContext(r.Context(), "request started",
				slog.String("method", r.Method),
				slog.String("path", path),
			)

			rec := &statusRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rec.Status)

			tel.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			observeDuration(tel.HTTPRequestDuration, r.Method, path, duration, traceID)

			slog.InfoContext(r.Context(), "request completed",
				slog.String("method", r.Method),
				slog.String("path", path),
				slog.Int("status", rec.Status),
				slog.Float64("duration_seconds", duration),
			)
		})
	}
}

func observeDuration(h *prometheus.HistogramVec, method, path string, seconds float64, traceID string) {
	observer := h.WithLabelValues(method, path)
	if ex, ok := observer.(prometheus.ExemplarObserver); ok && traceID != "" && traceID != "00000000000000000000000000000000" {
		ex.ObserveWithExemplar(seconds, prometheus.Labels{"trace_id": traceID})
		return
	}
	observer.Observe(seconds)
}

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.Status = code
	s.ResponseWriter.WriteHeader(code)
}
