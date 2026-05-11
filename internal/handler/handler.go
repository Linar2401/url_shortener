package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/database"
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
	SaveBatch(items []storage.BatchItem) error
}

type Pinger interface {
	Ping(ctx context.Context) error
}

type Handlers struct {
	storage URLStorer
	config  config.Config
	log     *zap.Logger
	pinger  Pinger
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

type BatchRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func Serve(cfg *config.Config) error {
	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	var urlStore URLStorer
	var pinger Pinger

	switch {
	case cfg.DatabaseDSN != "":
		db, err := database.New(cfg.DatabaseDSN)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.Error("failed to close database", zap.Error(err))
			}
		}()
		if err := db.Migrate(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		urlStore = db
		pinger = db
	case cfg.FileStoragePath != "":
		fs, err := storage.New(cfg.FileStoragePath)
		if err != nil {
			return fmt.Errorf("failed to initialize file storage: %w", err)
		}
		urlStore = fs
	default:
		fs, err := storage.New("")
		if err != nil {
			return fmt.Errorf("failed to initialize in-memory storage: %w", err)
		}
		urlStore = fs
	}

	handlers := New(urlStore, *cfg, log, pinger)

	log.Info("Running server", zap.String("address", cfg.ServeAddress))

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware(log))

	r.Method(http.MethodPost, "/", logger.RequestLogger(log, handlers.CreateHandle))
	r.Method(http.MethodGet, "/{code}", logger.RequestLogger(log, handlers.GetHandle))
	r.Method(http.MethodPost, "/api/shorten", logger.RequestLogger(log, handlers.ShortenJSONHandle))
	r.Method(http.MethodPost, "/api/shorten/batch", logger.RequestLogger(log, handlers.BatchHandle))
	r.Method(http.MethodGet, "/ping", logger.RequestLogger(log, handlers.PingHandle))

	return http.ListenAndServe(cfg.ServeAddress, r)
}

func New(storage URLStorer, cfg config.Config, log *zap.Logger, pinger Pinger) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
		log:     log,
		pinger:  pinger,
	}
}

func (h *Handlers) PingHandle(w http.ResponseWriter, r *http.Request) {
	if h.pinger == nil {
		h.log.Error("database is not configured")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := h.pinger.Ping(r.Context()); err != nil {
		h.log.Error("failed to ping database", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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

	status := http.StatusCreated
	shortURL, err := h.saveWithRetry(req.URL)
	if err != nil {
		var conflict *storage.ConflictError
		if errors.As(err, &conflict) {
			shortURL = conflict.ShortCode
			status = http.StatusConflict
		} else {
			h.log.Error("failed to save url", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		h.log.Error("failed to join result url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ShortenResponse{Result: resultURL}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
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

	status := http.StatusCreated
	shortURL, err := h.saveWithRetry(string(body))
	if err != nil {
		var conflict *storage.ConflictError
		if errors.As(err, &conflict) {
			shortURL = conflict.ShortCode
			status = http.StatusConflict
		} else {
			h.log.Error("failed to save url", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	resultURL, err := url.JoinPath(h.config.ResultAddress, shortURL)
	if err != nil {
		h.log.Error("failed to join result url", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	if _, err := w.Write([]byte(resultURL)); err != nil {
		h.log.Error("failed to write response body", zap.Error(err))
		return
	}
}

func (h *Handlers) BatchHandle(w http.ResponseWriter, r *http.Request) {
	var req []BatchRequestItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(req) == 0 {
		http.Error(w, "empty batch", http.StatusBadRequest)
		return
	}
	for _, item := range req {
		if item.OriginalURL == "" {
			http.Error(w, "original_url is required", http.StatusBadRequest)
			return
		}
	}

	saved, err := h.saveBatchWithRetry(req)
	if err != nil {
		h.log.Error("failed to save batch", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := make([]BatchResponseItem, len(saved))
	for i, item := range saved {
		shortURL, err := url.JoinPath(h.config.ResultAddress, item.ShortCode)
		if err != nil {
			h.log.Error("failed to join result url", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		resp[i] = BatchResponseItem{
			CorrelationID: req[i].CorrelationID,
			ShortURL:      shortURL,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
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
		code := generateCode()

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

func (h *Handlers) saveBatchWithRetry(req []BatchRequestItem) ([]storage.BatchItem, error) {
	for n := 0; n < maxTries; n++ {
		batch := make([]storage.BatchItem, len(req))
		for i, item := range req {
			batch[i] = storage.BatchItem{
				ShortCode:   generateCode(),
				OriginalURL: item.OriginalURL,
			}
		}
		err := h.storage.SaveBatch(batch)
		if err == nil {
			return batch, nil
		}
		if !errors.Is(err, storage.ErrCollision) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to save batch after %d tries", maxTries)
}

func generateCode() string {
	b := make([]byte, codeLen)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}
