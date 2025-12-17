package main

import (
	"net/http"

	main_handlers "github.com/Linar2401/url_shortener/internal/handlers"
	main_storage "github.com/Linar2401/url_shortener/internal/storage"
)

func main() {
	storage := main_storage.New()
	handlers := main_handlers.New(storage)

	mux := http.NewServeMux()
	mux.HandleFunc(`/`, handlers.CreateHandle)
	mux.HandleFunc(`/{code}`, handlers.GetHandle)

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
