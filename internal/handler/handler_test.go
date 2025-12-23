package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Linar2401/url_shortener/internal/config"
)

type MockStorage struct {
	data map[string]string
}

func (m *MockStorage) SaveURL(_ string) string {
	return "short123"
}

func (m *MockStorage) GetURL(code string) (string, bool) {
	url, ok := m.data[code]
	return url, ok
}

func TestHandlers_CreateHandle(t *testing.T) {
	cfg := config.NewConfig()

	type want struct {
		statusCode int
		response   string
	}
	tests := []struct {
		name   string
		method string
		body   string
		want   want
	}{
		{
			name:   "success",
			method: http.MethodPost,
			body:   "https://example.com",
			want: want{
				statusCode: http.StatusCreated,
				response:   "http://" + cfg.ResultAddress.String() + "/short123",
			},
		},
		{
			name:   "wrong method",
			method: http.MethodGet,
			body:   "https://example.com",
			want: want{
				statusCode: http.StatusBadRequest,
				response:   "Only POST\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{data: make(map[string]string)}
			h := New(storage, cfg.ServeAddress.String(), cfg.ResultAddress.String())

			r := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.CreateHandle(w, r)

			result := w.Result()
			err := result.Body.Close()
			if err != nil {
				t.Errorf("Error while closing body:")
			}

			if result.StatusCode != tt.want.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.want.statusCode, result.StatusCode)
			}

			body, _ := io.ReadAll(result.Body)
			if string(body) != tt.want.response {
				t.Errorf("Expected body %q, got %q", tt.want.response, string(body))
			}
		})
	}
}

func TestHandlers_GetHandle(t *testing.T) {
	cfg := config.NewConfig()

	type want struct {
		statusCode int
		location   string
	}
	tests := []struct {
		name    string
		method  string
		path    string
		storage map[string]string
		want    want
	}{
		{
			name:   "success",
			method: http.MethodGet,
			path:   "/short123",
			storage: map[string]string{
				"short123": "https://example.com",
			},
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				location:   "https://example.com",
			},
		},
		{
			name:    "not found",
			method:  http.MethodGet,
			path:    "/unknown",
			storage: map[string]string{},
			want: want{
				statusCode: http.StatusBadRequest,
				location:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{data: tt.storage}
			h := New(storage, cfg.ServeAddress.String(), cfg.ResultAddress.String())

			// Use ServeMux to handle PathValue extraction
			mux := http.NewServeMux()
			mux.HandleFunc("/{code}", h.GetHandle)

			r := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, r)

			result := w.Result()
			err := result.Body.Close()
			if err != nil {
				t.Errorf("Error while closing body:")
			}

			if result.StatusCode != tt.want.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.want.statusCode, result.StatusCode)
			}

			if tt.want.location != "" {
				loc := result.Header.Get("Location")
				if loc != tt.want.location {
					t.Errorf("Expected location %q, got %q", tt.want.location, loc)
				}
			}
		})
	}
}
