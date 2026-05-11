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

type FileRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type URLStore struct {
	mu              sync.Mutex
	codes           map[string]FileRecord
	fileStoragePath string
	uuidCounter     int
}

func New(fileStoragePath string) (*URLStore, error) {
	store := &URLStore{
		codes:           make(map[string]FileRecord),
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

func (s *URLStore) SaveURL(code string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.codes[code]; ok {
		return fmt.Errorf("%w: %s", ErrCollision, code)
	}

	s.uuidCounter++
	record := FileRecord{
		UUID:        strconv.Itoa(s.uuidCounter),
		ShortURL:    code,
		OriginalURL: value,
	}

	s.codes[code] = record

	if s.fileStoragePath != "" {
		if err := s.persist(); err != nil {
			return err
		}
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
