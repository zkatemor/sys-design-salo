package contract

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

var (
	loadDocOnce sync.Once
	loadedDoc   *openapi3.T
	loadedRoute routers.Router
	loadDocErr  error
)

func loadOpenAPI(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	loadDocOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		specPath := filepath.Join(filepath.Dir(file), "..", "..", "api", "openapi.yaml")
		raw, err := os.ReadFile(specPath)
		if err != nil {
			loadDocErr = fmt.Errorf("read openapi spec: %w", err)
			return
		}
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		doc, err := loader.LoadFromData(raw)
		if err != nil {
			loadDocErr = fmt.Errorf("parse openapi spec: %w", err)
			return
		}
		if err := doc.Validate(context.Background()); err != nil {
			loadDocErr = fmt.Errorf("validate openapi spec: %w", err)
			return
		}
		router, err := gorillamux.NewRouter(doc)
		if err != nil {
			loadDocErr = fmt.Errorf("build openapi router: %w", err)
			return
		}
		loadedDoc = doc
		loadedRoute = router
	})
	if loadDocErr != nil {
		t.Fatalf("load openapi: %v", loadDocErr)
	}
	return loadedDoc, loadedRoute
}

// assertResponse validates an HTTP response against the OpenAPI operation for req.
func assertResponse(t *testing.T, router routers.Router, req *http.Request, resp *http.Response, body []byte) {
	t.Helper()

	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("contract: operation not found in OpenAPI for %s %s: %v", req.Method, req.URL.Path, err)
	}

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			Route:      route,
			PathParams: pathParams,
		},
		Status: resp.StatusCode,
		Header: resp.Header,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Fatalf("contract violation for %s %s (status %d): %v\nbody: %s",
			req.Method, req.URL.Path, resp.StatusCode, err, truncate(body, 512))
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
