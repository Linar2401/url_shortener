package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// newBenchHandler builds a Handlers instance backed by the in-memory storage
// pre-seeded with `seed` shortened URLs so that GET / user-listing paths see
// a realistic working set instead of an empty map.
func newBenchHandler(b *testing.B, seed int) (*Handlers, *storage.URLStore, []string) {
	b.Helper()
	store, err := storage.New("")
	if err != nil {
		b.Fatalf("storage.New: %v", err)
	}
	codes := make([]string, 0, seed)
	for i := 0; i < seed; i++ {
		code := fmt.Sprintf("c%05d", i)
		if err := store.SaveURL(code, fmt.Sprintf("https://example.com/path/%d", i), "user-1"); err != nil {
			b.Fatalf("seed SaveURL: %v", err)
		}
		codes = append(codes, code)
	}
	cfg := config.NewDefaultConfig()
	h := New(store, *cfg, zap.NewNop(), nil, nil, nil)
	return h, store, codes
}

func BenchmarkCreateHandle(b *testing.B) {
	h, _, _ := newBenchHandler(b, 0)
	r := chi.NewRouter()
	r.Post("/", h.CreateHandle)

	body := "https://example.com/some/long/path/to/shorten"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkShortenJSONHandle(b *testing.B) {
	h, _, _ := newBenchHandler(b, 0)
	r := chi.NewRouter()
	r.Post("/api/shorten", h.ShortenJSONHandle)

	body := `{"url":"https://example.com/some/long/path/to/shorten"}`
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGetHandle(b *testing.B) {
	const seed = 10_000
	h, _, codes := newBenchHandler(b, seed)
	r := chi.NewRouter()
	r.Get("/{code}", h.GetHandle)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		code := codes[i%seed]
		req := httptest.NewRequest(http.MethodGet, "/"+code, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBatchHandle(b *testing.B) {
	h, _, _ := newBenchHandler(b, 0)
	r := chi.NewRouter()
	r.Post("/api/shorten/batch", h.BatchHandle)

	var sb strings.Builder
	sb.WriteByte('[')
	for i := 0; i < 20; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		_, _ = fmt.Fprintf(&sb, `{"correlation_id":"c%d","original_url":"https://example.com/p/%d"}`, i, i)
	}
	sb.WriteByte(']')
	body := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
