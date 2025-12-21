package main

import (
	"net/http"

	Handlers "github.com/Linar2401/url_shortener/internal/handler"
	Storages "github.com/Linar2401/url_shortener/internal/storage"
)

func main() {
	storage := Storages.New()
	handlers := Handlers.New(storage)

	mux := http.NewServeMux()
	mux.HandleFunc(`/`, handlers.CreateHandle)
	mux.HandleFunc(`/{code}`, handlers.GetHandle)

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
