package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"obssvc/internal/observability"
	"obssvc/internal/orders"
	"obssvc/internal/store"
)

func setupTestServer(t *testing.T) (*httptest.Server, *bytes.Buffer) {
	t.Helper()

	var logBuf bytes.Buffer

	tel, shutdown, err := observability.Setup(context.Background(), "obssvc-test")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	observability.SetLogOutput(&logBuf)
	t.Cleanup(func() {
		_ = shutdown(context.Background())
	})

	db := store.New()
	svc := orders.NewService(db, tel)
	h := NewHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("POST /orders", h.CreateOrder)
	mux.HandleFunc("GET /orders/{id}", h.GetOrder)
	mux.Handle("GET /metrics", promhttp.HandlerFor(tel.Reg, promhttp.HandlerOpts{}))

	handler := otelhttp.NewHandler(Middleware(tel)(mux), "http")
	return httptest.NewServer(handler), &logBuf
}

func TestHealthz(t *testing.T) {
	h := NewHandler(orders.NewService(store.New(), mustTelemetry(t)))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/orders", "application/json", strings.NewReader(`{"user":"bob","amount":42}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	metricsResp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer metricsResp.Body.Close()

	body, err := io.ReadAll(metricsResp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_active_connections",
		"orders_total",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics body missing %q", want)
		}
	}
}

func TestTraceIDInLogs(t *testing.T) {
	srv, logBuf := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/orders", "application/json", strings.NewReader(`{"user":"alice","amount":100}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	logs := logBuf.String()
	var found bool
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("invalid json log line: %q err=%v", line, err)
		}
		if tid, ok := entry["trace_id"].(string); ok && tid != "" && tid != "00000000000000000000000000000000" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no non-zero trace_id in logs:\n%s", logs)
	}
}

func TestTraceparentHeader(t *testing.T) {
	srv, _ := setupTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	tp := resp.Header.Get("Traceparent")
	if tp == "" {
		tp = resp.Header.Get("traceparent")
	}
	if tp == "" {
		t.Fatal("missing traceparent response header")
	}
	if !strings.HasPrefix(tp, "00-") {
		t.Fatalf("traceparent = %q, want W3C format 00-...", tp)
	}
}

func mustTelemetry(t *testing.T) *observability.Telemetry {
	t.Helper()
	tel, shutdown, err := observability.Setup(context.Background(), "obssvc-test")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return tel
}
