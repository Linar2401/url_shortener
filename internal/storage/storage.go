package storage

import (
	"math/rand/v2"
	"sync"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLen = 6
)

type URLStore struct {
	mu    sync.RWMutex
	codes map[string]string
}

func New() *URLStore {
	return &URLStore{
		codes: make(map[string]string),
	}
}

func (s *URLStore) SaveURL(value string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		if _, exists := s.codes[code]; !exists {
			s.codes[code] = value
			return code
		}
	}
}

func (s *URLStore) GetURL(code string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.codes[code]
	return val, ok
}
