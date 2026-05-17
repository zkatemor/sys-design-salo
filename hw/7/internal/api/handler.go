package api

import (
	"errors"
	"net/http"

	"cbservice/internal/breaker"
	"cbservice/internal/upstream"
)

type Handler struct {
	Upstream *upstream.Client
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /proxy", h.proxy)
	return mux
}

func (h *Handler) proxy(w http.ResponseWriter, r *http.Request) {
	body, err := h.Upstream.Fetch(r.Context())
	if err != nil {
		if errors.Is(err, breaker.ErrOpen) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(body))
}
