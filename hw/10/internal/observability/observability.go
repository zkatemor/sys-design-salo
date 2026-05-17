// Package observability — slog + OpenTelemetry + Prometheus.
package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry — tracer, registry и коллекторы метрик для middleware и сервисов.
type Telemetry struct {
	Tracer trace.Tracer
	Reg    *prometheus.Registry

	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPRequestDuration    *prometheus.HistogramVec
	HTTPActiveConnections  prometheus.Gauge
	OrdersTotal            *prometheus.CounterVec
}

type traceLogHandler struct {
	slog.Handler
}

func (h *traceLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceLogHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceLogHandler) WithGroup(name string) slog.Handler {
	return &traceLogHandler{Handler: h.Handler.WithGroup(name)}
}

// Setup инициализирует slog (JSON + trace_id/span_id), TracerProvider и Prometheus.
func Setup(ctx context.Context, serviceName string) (*Telemetry, func(context.Context) error, error) {
	exp, err := newTraceExporter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithHost(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(&traceLogHandler{Handler: jsonHandler}))

	tel := newTelemetry(serviceName)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx))
	}
	return tel, shutdown, nil
}

type noopExporter struct{}

func (noopExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return nil }
func (noopExporter) Shutdown(context.Context) error                             { return nil }

func newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(os.Getenv("OTEL_TRACES_EXPORTER")) {
	case "none", "noop", "off":
		return noopExporter{}, nil
	case "stdout", "console":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	default:
		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		endpoint = strings.TrimPrefix(endpoint, "http://")
		endpoint = strings.TrimPrefix(endpoint, "https://")

		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithURLPath("/v1/traces"),
		}
		if !strings.Contains(endpoint, "443") {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.New(ctx, opts...)
	}
}

func newTelemetry(serviceName string) *Telemetry {
	reg := prometheus.NewRegistry()

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	activeConns := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_active_connections",
		Help: "Number of active HTTP connections",
	})

	ordersTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "orders_total",
		Help: "Total orders created",
	}, []string{"status"})

	reg.MustRegister(
		httpRequests,
		httpDuration,
		activeConns,
		ordersTotal,
	)

	return &Telemetry{
		Tracer:                otel.Tracer(serviceName),
		Reg:                   reg,
		HTTPRequestsTotal:     httpRequests,
		HTTPRequestDuration:   httpDuration,
		HTTPActiveConnections: activeConns,
		OrdersTotal:           ordersTotal,
	}
}

// SetLogOutput перенаправляет JSON-логи (для тестов).
func SetLogOutput(w io.Writer) {
	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(&traceLogHandler{Handler: jsonHandler}))
}
