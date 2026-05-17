package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ctrsvc/internal/api"
	"ctrsvc/internal/orders"
	"ctrsvc/internal/users"
)

func startServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := &api.Handler{
		Users:  users.NewStore(),
		Orders: orders.NewStore(),
	}
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)
	return srv
}

func doRequest(t *testing.T, srv *httptest.Server, method, path string, body []byte) (*http.Request, *http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return req, resp, respBody
}

func TestContract_Healthz(t *testing.T) {
	_, router := loadOpenAPI(t)
	srv := startServer(t)

	req, resp, body := doRequest(t, srv, http.MethodGet, "/healthz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type: got %q want application/json", ct)
	}
	assertResponse(t, router, req, resp, body)

	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("поле status: got %q want ok", got["status"])
	}
}

func TestContract_CreateUser(t *testing.T) {
	_, router := loadOpenAPI(t)
	srv := startServer(t)

	// Happy path: 201 + User
	payload := []byte(`{"name":"Ada Lovelace","email":"ada@example.com"}`)
	req, resp, body := doRequest(t, srv, http.MethodPost, "/users", payload)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d want 201, body=%s", resp.StatusCode, body)
	}
	assertResponse(t, router, req, resp, body)

	var user struct {
		ID, Name, Email string
	}
	if err := json.Unmarshal(body, &user); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if user.ID == "" {
		t.Fatal("поле id отсутствует или пустое")
	}
	if user.Name != "Ada Lovelace" {
		t.Fatalf("поле name: got %q", user.Name)
	}
	if user.Email != "ada@example.com" {
		t.Fatalf("поле email: got %q", user.Email)
	}

	// Validation error: 400 plain text
	req2, resp2, body2 := doRequest(t, srv, http.MethodPost, "/users", []byte(`{"name":"","email":"x@y.z"}`))
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", resp2.StatusCode)
	}
	assertResponse(t, router, req2, resp2, body2)
	if !strings.Contains(string(body2), "required") {
		t.Fatalf("тело ошибки: got %q", body2)
	}
}

func TestContract_GetUser(t *testing.T) {
	_, router := loadOpenAPI(t)
	srv := startServer(t)

	// Seed user via API (same contract as consumers use)
	_, createResp, createBody := doRequest(t, srv, http.MethodPost, "/users",
		[]byte(`{"name":"Grace Hopper","email":"grace@example.com"}`))
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create status: %d body=%s", createResp.StatusCode, createBody)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createBody, &created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}

	// 200 — формат User совпадает с POST /users
	req, resp, body := doRequest(t, srv, http.MethodGet, "/users/"+created.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	assertResponse(t, router, req, resp, body)

	// 404
	reqNF, respNF, bodyNF := doRequest(t, srv, http.MethodGet, "/users/does-not-exist", nil)
	if respNF.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", respNF.StatusCode)
	}
	assertResponse(t, router, reqNF, respNF, bodyNF)
}

func TestContract_CreateOrder(t *testing.T) {
	_, router := loadOpenAPI(t)
	srv := startServer(t)

	_, uResp, uBody := doRequest(t, srv, http.MethodPost, "/users",
		[]byte(`{"name":"Buyer","email":"buyer@example.com"}`))
	if uResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user: %d %s", uResp.StatusCode, uBody)
	}
	var user struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(uBody, &user)

	orderReq := []byte(`{"user_id":"` + user.ID + `","items":[{"sku":"BOOK-1","qty":2}]}`)
	req, resp, body := doRequest(t, srv, http.MethodPost, "/orders", orderReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: got %d want 201, body=%s", resp.StatusCode, body)
	}
	assertResponse(t, router, req, resp, body)

	var order struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
		Total  int    `json:"total"`
		Items  []struct {
			SKU string `json:"sku"`
			Qty int    `json:"qty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &order); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if order.ID == "" {
		t.Fatal("поле id отсутствует или пустое")
	}
	if order.UserID != user.ID {
		t.Fatalf("поле user_id: got %q want %q", order.UserID, user.ID)
	}
	if order.Total != 200 {
		t.Fatalf("поле total: got %d want 200", order.Total)
	}
	if len(order.Items) != 1 || order.Items[0].SKU != "BOOK-1" {
		t.Fatalf("поле items: %+v", order.Items)
	}
}

func TestContract_GetOrder(t *testing.T) {
	_, router := loadOpenAPI(t)
	srv := startServer(t)

	_, uResp, uBody := doRequest(t, srv, http.MethodPost, "/users",
		[]byte(`{"name":"Buyer","email":"buyer@example.com"}`))
	var user struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(uBody, &user)
	if uResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user failed")
	}

	_, oResp, oBody := doRequest(t, srv, http.MethodPost, "/orders",
		[]byte(`{"user_id":"`+user.ID+`","items":[{"sku":"X","qty":1}]}`))
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(oBody, &created)
	if oResp.StatusCode != http.StatusCreated {
		t.Fatalf("create order failed: %s", oBody)
	}

	req, resp, body := doRequest(t, srv, http.MethodGet, "/orders/"+created.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	assertResponse(t, router, req, resp, body)

	reqNF, respNF, bodyNF := doRequest(t, srv, http.MethodGet, "/orders/missing-id", nil)
	if respNF.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d want 404", respNF.StatusCode)
	}
	assertResponse(t, router, reqNF, respNF, bodyNF)
}
