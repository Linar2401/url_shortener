package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

const (
	CookieName = "auth_token"
	idBytes    = 16
)

var (
	ErrInvalidToken = errors.New("invalid auth token")
	ErrEmptyUserID  = errors.New("empty user id")
)

type ctxKey int

const userIDCtxKey ctxKey = 0

func Sign(userID string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(userID))
	sig := mac.Sum(nil)
	return userID + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func Verify(token string, secret []byte) (string, error) {
	idx := strings.LastIndex(token, ".")
	if idx <= 0 {
		return "", ErrInvalidToken
	}
	userID := token[:idx]
	sig, err := base64.RawURLEncoding.DecodeString(token[idx+1:])
	if err != nil {
		return "", ErrInvalidToken
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(userID))
	want := mac.Sum(nil)
	if !hmac.Equal(sig, want) {
		return "", ErrInvalidToken
	}
	if userID == "" {
		return "", ErrEmptyUserID
	}
	return userID, nil
}

func newUserID() (string, error) {
	b := make([]byte, idBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func setCookie(w http.ResponseWriter, userID string, secret []byte) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    Sign(userID, secret),
		Path:     "/",
		HttpOnly: true,
	})
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDCtxKey).(string)
	return v
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDCtxKey, userID)
}

// Middleware ensures a user ID is set in the request context. If the request's
// cookie is missing or fails signature verification, a new user ID is generated
// and a fresh signed cookie is written on the response.
func Middleware(secret []byte, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var userID string
			if cookie, err := r.Cookie(CookieName); err == nil {
				if id, vErr := Verify(cookie.Value, secret); vErr == nil {
					userID = id
				}
			}
			if userID == "" {
				newID, err := newUserID()
				if err != nil {
					log.Error("failed to generate user id", zap.Error(err))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				userID = newID
				setCookie(w, userID, secret)
			}
			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
		})
	}
}
