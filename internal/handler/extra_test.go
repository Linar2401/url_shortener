package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Linar2401/url_shortener/internal/auth"
	"github.com/Linar2401/url_shortener/internal/config"
	"github.com/Linar2401/url_shortener/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func newRealHandler(t *testing.T) (*Handlers, *storage.URLStore) {
	t.Helper()
	store, err := storage.New("")
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	cfg := config.NewDefaultConfig()
	return New(store, *cfg, zap.NewNop(), nil, nil, nil), store
}

func TestBatchHandle_Success(t *testing.T) {
	h, _ := newRealHandler(t)
	r := chi.NewRouter()
	r.Post("/api/shorten/batch", h.BatchHandle)

	body := `[{"correlation_id":"c1","original_url":"https://a.example"},{"correlation_id":"c2","original_url":"https://b.example"}]`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d", w.Code)
	}
	var resp []BatchResponseItem
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 2 || resp[0].CorrelationID != "c1" || resp[1].CorrelationID != "c2" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestBatchHandle_InvalidInputs(t *testing.T) {
	h, _ := newRealHandler(t)
	r := chi.NewRouter()
	r.Post("/api/shorten/batch", h.BatchHandle)

	cases := []struct {
		body string
		want int
	}{
		{`not-json`, http.StatusBadRequest},
		{`[]`, http.StatusBadRequest},
		{`[{"correlation_id":"c1","original_url":""}]`, http.StatusBadRequest},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(c.body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("body=%s got=%d want=%d", c.body, w.Code, c.want)
		}
	}
}

func TestShortenJSONHandle_Conflict(t *testing.T) {
	h, store := newRealHandler(t)
	if err := store.SaveURL("preset1", "https://dup.example", "u"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/api/shorten", h.ShortenJSONHandle)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://dup.example"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
	var resp ShortenResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if !strings.HasSuffix(resp.Result, "/preset1") {
		t.Errorf("expected preset1 in result, got %s", resp.Result)
	}
}

func TestCreateHandle_Conflict(t *testing.T) {
	h, store := newRealHandler(t)
	if err := store.SaveURL("preset2", "https://dup2.example", "u"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := chi.NewRouter()
	r.Post("/", h.CreateHandle)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://dup2.example"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
	if !strings.HasSuffix(w.Body.String(), "/preset2") {
		t.Errorf("expected preset2 in body, got %s", w.Body.String())
	}
}

func TestGetHandle_Deleted(t *testing.T) {
	h, store := newRealHandler(t)
	_ = store.SaveURL("gone1", "https://x.example", "u")
	_ = store.DeleteUserURLs([]string{"gone1"}, "u")

	r := chi.NewRouter()
	r.Get("/{code}", h.GetHandle)

	req := httptest.NewRequest(http.MethodGet, "/gone1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("want 410, got %d", w.Code)
	}
}

func TestUserURLsHandle(t *testing.T) {
	h, store := newRealHandler(t)
	_ = store.SaveURL("ua", "https://a.example", "alice")
	_ = store.SaveURL("ub", "https://b.example", "alice")

	secret := []byte("k")
	r := chi.NewRouter()
	r.Get("/api/user/urls", h.UserURLsHandle(secret))

	t.Run("unauthorized when no context user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", w.Code)
		}
	})

	t.Run("returns urls when user present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		req = req.WithContext(auth.WithUserID(req.Context(), "alice"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", w.Code)
		}
		var resp []UserURLItem
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp) != 2 {
			t.Errorf("want 2 urls, got %d", len(resp))
		}
	})

	t.Run("no content when user has none", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		req = req.WithContext(auth.WithUserID(req.Context(), "no-one"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Errorf("want 204, got %d", w.Code)
		}
	})
}
