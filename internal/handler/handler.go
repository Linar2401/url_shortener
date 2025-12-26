package handler

import (
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type URLStorer interface {
	SaveURL(value string) string
	GetURL(code string) (string, error)
}

type Handlers struct {
	storage URLStorer
	config  config.Config
}

func Serve(cfg *config.Config) error {
	r := chi.NewRouter()

	urlStore := storage.New()
	handlers := New(urlStore, *cfg)

	r.Use(middleware.Logger)

	r.Post("/", handlers.CreateHandle)
	r.Get("/{code}", handlers.GetHandle)

	return http.ListenAndServe(cfg.ServeAddress, r)
}

func New(storage URLStorer, cfg config.Config) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
	}
}

func (h *Handlers) CreateHandle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Fatalln(http.StatusText(http.StatusInternalServerError))
		return
	}

	shortURL := h.storage.SaveURL(string(body))

	resultUrl, _ := url.JoinPath(h.config.ResultAddress, shortURL)
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(resultUrl))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Fatalln(http.StatusText(http.StatusInternalServerError))
		return
	}
}

func (h *Handlers) GetHandle(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	val, err := h.storage.GetURL(code)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		log.Println("Code not found")
		return
	}

	http.Redirect(w, r, val, http.StatusTemporaryRedirect)
}
