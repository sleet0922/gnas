package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jeessy2/gnas/internal/db"
)

type contextKey string

const usernameContextKey contextKey = "username"

// jwtSecret 在 InitAuth 中从 DB 加载或生成新密钥后赋值
var jwtSecret []byte

// tokenBlacklist 内存 token 黑名单，key: token string, value: 过期时间
var tokenBlacklist sync.Map

type tokenClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func newJwtSecret() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		log.Fatal("failed to generate JWT secret")
	}
	return secret
}

// InitAuth 从 DB 加载 JWT 密钥，若不存在则生成新密钥并持久化
func InitAuth() {
	stored, err := db.GetSetting("jwt_secret")
	if err != nil {
		log.Fatalf("[Auth] 读取 JWT 密钥失败: %v", err)
	}
	if stored != "" {
		if decoded, err := base64.StdEncoding.DecodeString(stored); err == nil && len(decoded) == 32 {
			jwtSecret = decoded
			return
		}
	}
	secret := newJwtSecret()
	if err := db.SetSetting("jwt_secret", base64.StdEncoding.EncodeToString(secret)); err != nil {
		log.Fatalf("[Auth] 保存 JWT 密钥失败: %v", err)
	}
	jwtSecret = secret
}

// RevokeToken 解析 token 的过期时间并加入黑名单
func RevokeToken(token string) {
	claims := &tokenClaims{}
	_, _, err := jwt.NewParser().ParseUnverified(token, claims)
	var expiry time.Time
	if err == nil && claims.ExpiresAt != nil {
		expiry = claims.ExpiresAt.Time
	} else {
		expiry = time.Now().Add(24 * time.Hour)
	}
	tokenBlacklist.Store(token, expiry)
	cleanupTokenBlacklist()
}

// cleanupTokenBlacklist 删除已过期的黑名单条目
func cleanupTokenBlacklist() {
	now := time.Now()
	tokenBlacklist.Range(func(key, value interface{}) bool {
		if t, ok := value.(time.Time); ok && !t.After(now) {
			tokenBlacklist.Delete(key)
		}
		return true
	})
}

func issueToken(username string) (string, error) {
	claims := tokenClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func validateToken(raw string) (string, error) {
	// 检查 token 是否在黑名单中（且未过期）
	if expiry, ok := tokenBlacklist.Load(raw); ok {
		if t, ok := expiry.(time.Time); ok && t.After(time.Now()) {
			return "", jwt.ErrTokenInvalidClaims
		}
		tokenBlacklist.Delete(raw)
	}
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
