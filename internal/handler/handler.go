package handler

import (
	"fmt"
	"io"
	"net/http"

	config "github.com/Linar2401/url_shortener/internal/config"
)

type URLStorer interface {
	SaveURL(value string) string
	GetURL(code string) (string, bool)
}

type Handlers struct {
	storage URLStorer
	config  config.Config
}

func New(storage URLStorer, cfg config.Config) *Handlers {
	return &Handlers{
		storage: storage,
		config:  cfg,
	}
}

func (h *Handlers) CreateHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	shortURL := h.storage.SaveURL(string(body))

	result := fmt.Sprintf("%s/%s", h.config.ResultAddress, shortURL)
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(result))
	if err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) GetHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET", http.StatusBadRequest)
		return
	}
	code := r.PathValue("code")

	val, ok := h.storage.GetURL(code)

	if !ok {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, val, http.StatusTemporaryRedirect)
}
