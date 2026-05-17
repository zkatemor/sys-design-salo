package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"ctrsvc/internal/orders"
	"ctrsvc/internal/users"
)

type Handler struct {
	Users  *users.Store
	Orders *orders.Store
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /users", h.listUsers)
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("GET /users/{id}", h.getUser)
	mux.HandleFunc("POST /orders", h.createOrder)
	mux.HandleFunc("GET /orders/{id}", h.getOrder)
	return mux
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listUsers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Users.List())
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Email == "" {
		http.Error(w, "name and email required", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, h.Users.Create(req.Name, req.Email))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.Users.Get(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string        `json:"user_id"`
		Items  []orders.Item `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || len(req.Items) == 0 {
		http.Error(w, "user_id and items required", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, h.Orders.Create(req.UserID, req.Items))
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	o, err := h.Orders.Get(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, orders.ErrNotFound) {
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
