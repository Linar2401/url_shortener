package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/logger"
	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
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

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

func Serve(cfg *config.Config) error {
	r := chi.NewRouter()

	urlStore := storage.New()
	handlers := New(urlStore, *cfg)

	if err := logger.Initialize(cfg.LogLevel); err != nil {
		return err
	}

	logger.Log.Info("Running server", zap.String("address", cfg.ServeAddress))
	// оборачиваем хендлер webhook в middleware с логированием

	//r.Use(middleware.Logger)

	r.Method(http.MethodPost, "/", logger.RequestLogger(handlers.CreateHandle))
	r.Method(http.MethodGet, "/{code}", logger.RequestLogger(handlers.GetHandle))
	r.Method(http.MethodPost, "/api/shorten", logger.RequestLogger(handlers.ShortenJSONHandle))

	return http.ListenAndServe(cfg.ServeAddress, r)
}

func New(storage URLStorer, cfg config.Config) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
	}
}

func (h *Handlers) ShortenJSONHandle(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	originalURL := req.URL

	var shortURL string
	for n := 0; n < maxTries; n++ {
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		err := h.storage.SaveURL(code, originalURL)
		if err == nil {
			shortURL = code
			break
		}
		if !errors.Is(err, storage.ErrCollision) {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			log.Println("error with save url")
			return
		}
	}

	if shortURL == "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with save url: max tries reached")
		return
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with join path")
		return
	}

	res := ShortenResponse{Result: resultURL}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with write response body")
		return
	}
}

func (h *Handlers) CreateHandle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with read body")
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
			log.Println("error with save url")
			return
		}
	}

	if shortURL == "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with save url: max tries reached")
		return
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with join path")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(resultURL))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		log.Println("error with write response body")
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
