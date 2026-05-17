// Учебный сервис: users + orders.
//
// ENV:
//   API_ADDR — адрес сервиса (по умолчанию :8080)
//
// Эндпоинты:
//   GET  /healthz
//   GET  /users
//   POST /users        {"name":"...","email":"..."}
//   GET  /users/{id}
//   POST /orders       {"user_id":"...","items":[{"sku":"...","qty":1}]}
//   GET  /orders/{id}
package main

import (
	"cmp"
	"log"
	"net/http"
	"os"
	"time"

	"ctrsvc/internal/api"
	"ctrsvc/internal/orders"
	"ctrsvc/internal/users"
)

func main() {
	addr := cmp.Or(os.Getenv("API_ADDR"), ":8080")

	h := &api.Handler{
		Users:  users.NewStore(),
		Orders: orders.NewStore(),
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
