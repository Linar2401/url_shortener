package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
)

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
				response:   cfg.ResultAddress + "/short123",
			},
		},
		{
			name:   "wrong method",
			method: http.MethodGet,
			body:   "https://example.com",
			want: want{
				statusCode: http.StatusMethodNotAllowed,
				response:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockURLStorer(t)

			if tt.method == http.MethodPost {
				storage.On("SaveURL", mock.Anything).Return("short123")
			}

			h := New(storage, *cfg)

			r := chi.NewRouter()
			r.Post("/", h.CreateHandle)

			req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			result := w.Result()
			err := result.Body.Close()
			if err != nil {
				t.Errorf("Error while closing body:")
			}

			if result.StatusCode != tt.want.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.want.statusCode, result.StatusCode)
			}

			if tt.want.response != "" {
				body, _ := io.ReadAll(result.Body)
				if string(body) != tt.want.response {
					t.Errorf("Expected body %q, got %q", tt.want.response, string(body))
				}
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
		name         string
		method       string
		path         string
		mockBehavior func(s *MockURLStorer)
		want         want
	}{
		{
			name:   "success",
			method: http.MethodGet,
			path:   "/short123",
			mockBehavior: func(s *MockURLStorer) {
				s.On("GetURL", "short123").Return("https://example.com", nil)
			},
			want: want{
				statusCode: http.StatusTemporaryRedirect,
				location:   "https://example.com",
			},
		},
		{
			name:   "not found",
			method: http.MethodGet,
			path:   "/unknown",
			mockBehavior: func(s *MockURLStorer) {
				s.On("GetURL", "unknown").Return("", errors.New("not found"))
			},
			want: want{
				statusCode: http.StatusNotFound,
				location:   "",
			},
		},
		{
			name:   "wrong method",
			method: http.MethodPost,
			path:   "/unknown",
			mockBehavior: func(s *MockURLStorer) {
			},
			want: want{
				statusCode: http.StatusMethodNotAllowed,
				location:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewMockURLStorer(t)
			if tt.mockBehavior != nil {
				tt.mockBehavior(storage)
			}

			h := New(storage, *cfg)

			r := chi.NewRouter()
			r.Get("/{code}", h.GetHandle)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

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
