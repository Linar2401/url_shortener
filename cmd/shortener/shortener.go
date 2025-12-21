package main

import (
	"net/http"

	Handlers "github.com/Linar2401/url_shortener/internal/handler"
	Storages "github.com/Linar2401/url_shortener/internal/storage"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()

	storage := Storages.New()
	handlers := Handlers.New(storage)

	r.Use(middleware.Logger)

	r.Post("/", handlers.CreateHandle)
	r.Get("/{code}", handlers.GetHandle)

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		return
	}
}
