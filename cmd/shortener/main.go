package main

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLen = 6
)

type URLStore struct {
	mu    sync.RWMutex      // RWMutex: много читателей, один писатель
	codes map[string]string // Храним Код -> Строка
}

func (s *URLStore) SaveURL(value string) string {
	s.mu.Lock() // Блокируем на запись
	defer s.mu.Unlock()

	for {
		// Генерируем случайную строку
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		// Проверяем на уникальность: если такого ключа нет — сохраняем
		if _, exists := s.codes[code]; !exists {
			s.codes[code] = value
			return code
		}
		// Если код занят, цикл пойдет на новый круг
	}
}

func (s *URLStore) GetURL(code string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.codes[code]
	return val, ok
}

func (s *URLStore) CreateHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}

	shortURL := s.SaveURL(string(body))

	result := fmt.Sprintf("http://localhost:8080/%s", shortURL)

	_, err = w.Write([]byte(result))
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *URLStore) GetHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET", http.StatusBadRequest)
		return
	}
	code := r.PathValue("code")

	s.mu.RLock()
	val, ok := s.codes[code]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "Код не найден", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, val, http.StatusFound)
}

func main() {
	storage := &URLStore{
		codes: make(map[string]string),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(`/`, storage.CreateHandle)
	mux.HandleFunc(`/{code}`, storage.GetHandle)

	err := http.ListenAndServe(`:8080`, mux)
	if err != nil {
		panic(err)
	}
}
