package main

import (
	"net/http"

	Handlers "github.com/Linar2401/url_shortener/internal/handler"
	Storages "github.com/Linar2401/url_shortener/internal/storage"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	config := parseFlags()

	if err := run(config); err != nil {
		panic(err)
	}
}

func run(config DefaultConfig) error {
	r := chi.NewRouter()

	storage := Storages.New()
	handlers := Handlers.New(storage, config.ServeAddress.String(), config.ResultAddress.String())

	r.Use(middleware.Logger)

	r.Post("/", handlers.CreateHandle)
	r.Get("/{code}", handlers.GetHandle)

	return http.ListenAndServe(config.ServeAddress.String(), r)
}
