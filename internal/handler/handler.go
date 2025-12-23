package handler

import (
	"fmt"
	"io"
	"net/http"
)

type URLStorer interface {
	SaveURL(value string) string
	GetURL(code string) (string, bool)
}

type Handlers struct {
	storage    URLStorer
	runAddr    string
	resultAddr string
}

func New(storage URLStorer, runAddr string, resultAddr string) *Handlers {
	return &Handlers{
		storage:    storage,
		runAddr:    runAddr,
		resultAddr: resultAddr,
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

	result := fmt.Sprintf("%s/%s", h.resultAddr, shortURL)
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
