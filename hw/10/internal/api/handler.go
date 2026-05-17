package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"obssvc/internal/orders"
	"obssvc/internal/store"
)

type Handler struct {
	Svc *orders.Service
}

func NewHandler(svc *orders.Service) *Handler {
	return &Handler{Svc: svc}
}

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok\n"))
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User   string `json:"user"`
		Amount int    `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	slog.InfoContext(r.Context(), "create order",
		slog.String("user", req.User),
		slog.Int("amount", req.Amount),
	)

	o, err := h.Svc.Create(r.Context(), req.User, req.Amount)
	if err != nil {
		slog.ErrorContext(r.Context(), "create order failed", slog.Any("err", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	o, err := h.Svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
