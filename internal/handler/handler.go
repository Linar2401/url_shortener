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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Linar2401/url_shortener/internal/auth"
	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/database"
	"github.com/Linar2401/url_shortener/internal/deleter"
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
	SaveURL(code string, value string, userID string) error
	GetURL(code string) (string, error)
	SaveBatch(items []storage.BatchItem, userID string) error
	GetUserURLs(userID string) ([]storage.UserURL, error)
	DeleteUserURLs(codes []string, userID string) error
}

type Pinger interface {
	Ping(ctx context.Context) error
}

// Enqueuer is the deleter-side dependency the DELETE handler needs.
type Enqueuer interface {
	Enqueue(codes []string, userID string)
}

type Handlers struct {
	storage URLStorer
	config  config.Config
	log     *zap.Logger
	pinger  Pinger
	deleter Enqueuer
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

type UserURLItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
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

	del := deleter.New(urlStore, log, 64, time.Second)
	del.Start()

	handlers := New(urlStore, *cfg, log, pinger, del)

	log.Info("Running server", zap.String("address", cfg.ServeAddress))

	secret := []byte(cfg.AuthSecret)

	r := chi.NewRouter()
	r.Use(middleware.GzipMiddleware(log))
	r.Use(auth.Middleware(secret, log))

	r.Method(http.MethodPost, "/", logger.RequestLogger(log, handlers.CreateHandle))
	r.Method(http.MethodGet, "/{code}", logger.RequestLogger(log, handlers.GetHandle))
	r.Method(http.MethodPost, "/api/shorten", logger.RequestLogger(log, handlers.ShortenJSONHandle))
	r.Method(http.MethodPost, "/api/shorten/batch", logger.RequestLogger(log, handlers.BatchHandle))
	r.Method(http.MethodGet, "/api/user/urls", logger.RequestLogger(log, handlers.UserURLsHandle(secret)))
	r.Method(http.MethodDelete, "/api/user/urls", logger.RequestLogger(log, handlers.DeleteUserURLsHandle(secret)))
	r.Method(http.MethodGet, "/ping", logger.RequestLogger(log, handlers.PingHandle))

	srv := &http.Server{Addr: cfg.ServeAddress, Handler: r}

	idleClosed := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server shutdown error", zap.Error(err))
		}
		del.Stop(shutdownCtx)
		close(idleClosed)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-idleClosed
	return nil
}

func New(storage URLStorer, cfg config.Config, log *zap.Logger, pinger Pinger, deleter Enqueuer) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
		log:     log,
		pinger:  pinger,
		deleter: deleter,
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

	userID := auth.UserIDFromContext(r.Context())

	status := http.StatusCreated
	shortURL, err := h.saveWithRetry(req.URL, userID)
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

	userID := auth.UserIDFromContext(r.Context())

	status := http.StatusCreated
	shortURL, err := h.saveWithRetry(string(body), userID)
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

	userID := auth.UserIDFromContext(r.Context())

	saved, err := h.saveBatchWithRetry(req, userID)
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
		if errors.Is(err, storage.ErrDeleted) {
			http.Error(w, http.StatusText(http.StatusGone), http.StatusGone)
			return
		}
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	http.Redirect(w, r, val, http.StatusTemporaryRedirect)
}

// UserURLsHandle returns a handler that lists URLs owned by the authenticated user.
// secret is the same key used by the auth middleware; it lets the handler enforce
// the strict 401 contract when the incoming cookie cannot be verified into a user ID.
func (h *Handlers) UserURLsHandle(secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.CookieName); err == nil {
			if _, vErr := auth.Verify(cookie.Value, secret); vErr != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}

		userID := auth.UserIDFromContext(r.Context())
		if userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		urls, err := h.storage.GetUserURLs(userID)
		if err != nil {
			h.log.Error("failed to fetch user urls", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if len(urls) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		resp := make([]UserURLItem, len(urls))
		for i, item := range urls {
			shortURL, err := url.JoinPath(h.config.ResultAddress, item.ShortCode)
			if err != nil {
				h.log.Error("failed to join result url", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			resp[i] = UserURLItem{
				ShortURL:    shortURL,
				OriginalURL: item.OriginalURL,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			h.log.Error("failed to write response body", zap.Error(err))
			return
		}
	}
}

// DeleteUserURLsHandle accepts a JSON array of short codes and asynchronously
// marks them as deleted for the authenticated user. Returns 202 immediately.
func (h *Handlers) DeleteUserURLsHandle(secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.CookieName); err == nil {
			if _, vErr := auth.Verify(cookie.Value, secret); vErr != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
		}

		userID := auth.UserIDFromContext(r.Context())
		if userID == "" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		var codes []string
		if err := json.NewDecoder(r.Body).Decode(&codes); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if h.deleter != nil && len(codes) > 0 {
			h.deleter.Enqueue(codes, userID)
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handlers) saveWithRetry(originalURL string, userID string) (string, error) {
	for n := 0; n < maxTries; n++ {
		code := generateCode()

		err := h.storage.SaveURL(code, originalURL, userID)
		if err == nil {
			return code, nil
		}
		if !errors.Is(err, storage.ErrCollision) {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to generate unique short URL after %d tries", maxTries)
}

func (h *Handlers) saveBatchWithRetry(req []BatchRequestItem, userID string) ([]storage.BatchItem, error) {
	for n := 0; n < maxTries; n++ {
		batch := make([]storage.BatchItem, len(req))
		for i, item := range req {
			batch[i] = storage.BatchItem{
				ShortCode:   generateCode(),
				OriginalURL: item.OriginalURL,
			}
		}
		err := h.storage.SaveBatch(batch, userID)
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
