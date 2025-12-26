package handler

import (
	"errors"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	charset  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLen  = 6
	maxTries = 100
)

type URLStorer interface {
	SaveURL(code string, value string) error
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
		log.Fatalln("error with read body")
		return
	}

	var shortURL string
	shortURL = ""
	for n := 0; n < maxTries; n++ {
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		err = h.storage.SaveURL(code, string(body))
		if err == nil {
			shortURL = code
			break
		}
		if !errors.Is(err, storage.ErrCollision) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			log.Fatalln("error with save url")
			return
		}
	}

	if shortURL == "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Fatalln("error with save url: max tries reached")
		return
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Fatalln("error with join path")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(resultURL))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Fatalln("error with write response body")
		return
	}
}

func (h *Handlers) GetHandle(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	val, err := h.storage.GetURL(code)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	http.Redirect(w, r, val, http.StatusTemporaryRedirect)
}
