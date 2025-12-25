package storage

import (
	"errors"
	"math/rand/v2"
	"sync"
)

const (
	charset  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLen  = 6
	maxTries = 100
)

type URLStore struct {
	mu    sync.Mutex
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

	for n := 0; n < maxTries; n++ {
		b := make([]byte, codeLen)
		for i := range b {
			b[i] = charset[rand.IntN(len(charset))]
		}
		code := string(b)

		if _, ok := s.codes[code]; !ok {
			s.codes[code] = value
			return code
		}
	}
	return ""
}

func (s *URLStore) GetURL(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.codes[code]
	if !ok {
		return "", errors.New("url not found")
	}
	return val, nil
}
