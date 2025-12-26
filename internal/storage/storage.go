package storage

import (
	"errors"
	"sync"
)

var ErrCollision = errors.New("collision")

type URLStore struct {
	mu    sync.Mutex
	codes map[string]string
}

func New() *URLStore {
	return &URLStore{
		codes: make(map[string]string),
	}
}

func (s *URLStore) SaveURL(code string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.codes[code]; ok {
		return ErrCollision
	}

	s.codes[code] = value
	return nil
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
