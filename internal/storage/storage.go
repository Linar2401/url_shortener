package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
)

var ErrCollision = errors.New("collision")

// ConflictError signals that the original URL is already stored under ShortCode.
type ConflictError struct {
	ShortCode string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("original url already shortened as %s", e.ShortCode)
}

type FileRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
}

type BatchItem struct {
	ShortCode   string
	OriginalURL string
}

type UserURL struct {
	ShortCode   string
	OriginalURL string
}

type URLStore struct {
	mu              sync.Mutex
	codes           map[string]FileRecord
	originals       map[string]string
	fileStoragePath string
	uuidCounter     int
}

func New(fileStoragePath string) (*URLStore, error) {
	store := &URLStore{
		codes:           make(map[string]FileRecord),
		originals:       make(map[string]string),
		fileStoragePath: fileStoragePath,
		uuidCounter:     0,
	}

	if fileStoragePath != "" {
		if err := store.loadFromFile(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (s *URLStore) loadFromFile() error {
	file, err := os.OpenFile(s.fileStoragePath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() == 0 {
		return nil
	}

	var records []FileRecord
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&records); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	for _, record := range records {
		s.codes[record.ShortURL] = record
		s.originals[record.OriginalURL] = record.ShortURL
		uuid, err := strconv.Atoi(record.UUID)
		if err == nil && uuid > s.uuidCounter {
			s.uuidCounter = uuid
		}
	}
	return nil
}

func (s *URLStore) persist() error {
	file, err := os.OpenFile(s.fileStoragePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	var records []FileRecord
	for _, record := range s.codes {
		records = append(records, record)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(records)
}

func (s *URLStore) SaveURL(code string, value string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.originals[value]; ok {
		return &ConflictError{ShortCode: existing}
	}

	if _, ok := s.codes[code]; ok {
		return fmt.Errorf("%w: %s", ErrCollision, code)
	}

	s.uuidCounter++
	record := FileRecord{
		UUID:        strconv.Itoa(s.uuidCounter),
		ShortURL:    code,
		OriginalURL: value,
		UserID:      userID,
	}

	s.codes[code] = record
	s.originals[value] = code

	if s.fileStoragePath != "" {
		if err := s.persist(); err != nil {
			return err
		}
	}

	return nil
}

func (s *URLStore) SaveBatch(items []BatchItem, userID string) error {
	if len(items) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range items {
		if existing, ok := s.originals[item.OriginalURL]; ok {
			items[i].ShortCode = existing
			continue
		}
		if _, ok := s.codes[item.ShortCode]; ok {
			return fmt.Errorf("%w: %s", ErrCollision, item.ShortCode)
		}
	}

	for _, item := range items {
		if _, ok := s.codes[item.ShortCode]; ok {
			continue
		}
		s.uuidCounter++
		s.codes[item.ShortCode] = FileRecord{
			UUID:        strconv.Itoa(s.uuidCounter),
			ShortURL:    item.ShortCode,
			OriginalURL: item.OriginalURL,
			UserID:      userID,
		}
		s.originals[item.OriginalURL] = item.ShortCode
	}

	if s.fileStoragePath != "" {
		return s.persist()
	}
	return nil
}

func (s *URLStore) GetURL(code string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.codes[code]
	if !ok {
		return "", errors.New("url not found")
	}
	return record.OriginalURL, nil
}

func (s *URLStore) GetUserURLs(userID string) ([]UserURL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []UserURL
	for _, record := range s.codes {
		if record.UserID == userID {
			result = append(result, UserURL{
				ShortCode:   record.ShortURL,
				OriginalURL: record.OriginalURL,
			})
		}
	}
	return result, nil
}
