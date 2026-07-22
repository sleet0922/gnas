package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const usernameContextKey contextKey = "username"

var jwtSecret = newJWTSecret()

type tokenClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func newJWTSecret() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err == nil {
		return secret
	}
	return []byte(time.Now().Format(time.RFC3339Nano))
}

func issueToken(username string) (string, error) {
	claims := tokenClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func validateToken(raw string) (string, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", jwt.ErrTokenInvalidClaims
	}
	if claims.Username == "" {
		return "", jwt.ErrTokenInvalidClaims
	}
	return claims.Username, nil
}

func tokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, err := validateToken(tokenFromRequest(r))
		if err != nil {
			writeErrorStatus(w, http.StatusUnauthorized, "token 缺失或无效")
			return
		}
		ctx := context.WithValue(r.Context(), usernameContextKey, username)
		next(w, r.WithContext(ctx))
	}
}

func CurrentUsername(r *http.Request) string {
	username, _ := r.Context().Value(usernameContextKey).(string)
	return username
}

func WithAccessControl(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func publicTokenForLog(token string) string {
	if token == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(token[:min(len(token), 8)]))
}
