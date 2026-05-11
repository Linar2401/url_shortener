package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/logger"
	"github.com/Linar2401/url_shortener/internal/middleware"
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
	log     *zap.Logger
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

func Serve(cfg *config.Config) error {
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	urlStore, err := storage.New(cfg.FileStoragePath)
	if err != nil {
		return fmt.Errorf("failed to initialize a URL store: %w", err)
	}
	handlers := New(urlStore, *cfg, log)

	log.Info("Running server", zap.String("address", cfg.ServeAddress))

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware(log))

	r.Method(http.MethodPost, "/", logger.RequestLogger(log, handlers.CreateHandle))
	r.Method(http.MethodGet, "/{code}", logger.RequestLogger(log, handlers.GetHandle))
	r.Method(http.MethodPost, "/api/shorten", logger.RequestLogger(log, handlers.ShortenJSONHandle))

	return http.ListenAndServe(cfg.ServeAddress, r)
}

func New(storage URLStorer, cfg config.Config, log *zap.Logger) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
		log:     log,
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

	shortURL, err := h.saveWithRetry(req.URL)
	if err != nil {
		h.log.Error("failed to save url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		h.log.Error("failed to join result url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ShortenResponse{Result: resultURL}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(res); err != nil {
		h.log.Error("failed to write response body", zap.Error(err))
		return
	}
}

func (h *Handlers) CreateHandle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error("failed to read request body", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	shortURL, err := h.saveWithRetry(string(body))
	if err != nil {
		h.log.Error("failed to save url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		h.log.Error("failed to join result url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if _, err := w.Write([]byte(resultURL)); err != nil {
		h.log.Error("failed to write response body", zap.Error(err))
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

func (h *Handlers) saveWithRetry(originalURL string) (string, error) {
	for n := 0; n < maxTries; n++ {
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		err := h.storage.SaveURL(code, originalURL)
		if err == nil {
			return code, nil
		}
		if !errors.Is(err, storage.ErrCollision) {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to generate unique short URL after %d tries", maxTries)
}
