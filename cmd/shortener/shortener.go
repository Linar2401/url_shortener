package main

import (
	"net/http"

	"github.com/Linar2401/url_shortener/internal/config"
	Handlers "github.com/Linar2401/url_shortener/internal/handler"
	Storages "github.com/Linar2401/url_shortener/internal/storage"
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	if err := run(cfg); err != nil {
		panic(err)
	}
}

func run(cfg *config.Config) error {
	r := chi.NewRouter()

	storage := Storages.New()
	handlers := Handlers.New(storage, cfg.ServeAddress.String(), cfg.ResultAddress.String())

	r.Use(middleware.Logger)

	r.Post("/", handlers.CreateHandle)
	r.Get("/{code}", handlers.GetHandle)

	return http.ListenAndServe(cfg.ServeAddress.String(), r)
}
