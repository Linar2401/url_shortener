package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestSignVerify_RoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	token := Sign("user-42", secret)
	got, err := Verify(token, secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != "user-42" {
		t.Errorf("want user-42, got %s", got)
	}
}

func TestVerify_BadInputs(t *testing.T) {
	secret := []byte("test-secret")

	cases := []struct {
		name  string
		token string
	}{
		{"no dot", "abcdef"},
		{"leading dot", ".sig"},
		{"bad base64", "uid.not_base64!!!"},
		{"wrong signature", "uid." + Sign("other", secret)[len("other")+1:]},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Verify(c.token, secret); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestUserIDContextHelpers(t *testing.T) {
	if got := UserIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty user id, got %q", got)
	}
	ctx := WithUserID(context.Background(), "user-7")
	if got := UserIDFromContext(ctx); got != "user-7" {
		t.Errorf("want user-7, got %q", got)
	}
}

func TestMiddleware_IssuesCookieWhenMissing(t *testing.T) {
	secret := []byte("k")
	var captured string
	handler := Middleware(secret, zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == "" {
		t.Error("expected user id to be set in context")
	}
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == CookieName {
			found = true
			if c.Value == "" {
				t.Error("cookie has empty value")
			}
		}
	}
	if !found {
		t.Error("expected auth cookie to be set")
	}
}

func TestMiddleware_AcceptsValidCookie(t *testing.T) {
	secret := []byte("k")
	signed := Sign("returning-user", secret)

	var captured string
	handler := Middleware(secret, zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: signed})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured != "returning-user" {
		t.Errorf("want returning-user, got %q", captured)
	}
}

func TestMiddleware_ReissuesOnBadCookie(t *testing.T) {
	secret := []byte("k")
	var captured string
	handler := Middleware(secret, zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "garbage.value"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if captured == "" {
		t.Error("expected fresh user id after invalid cookie")
	}
}
