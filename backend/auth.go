package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 配置 ====================

type Config struct {
	Port          string
	MySQLDSN      string
	ClickHouseDSN string
	JWTSecret     []byte
	AdminInitPass string // 初始 admin 密码（无则自动生成并打印）
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	LockThreshold int
	LockMinutes   int
	RetentionDays int // 日志留存天数（等保三级：180 天）
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func loadConfig() *Config {
	return &Config{
		Port:          env("PORT", "8080"),
		MySQLDSN:      env("MYSQL_DSN", "audit:audit2026@tcp(127.0.0.1:3306)/audit_meta?parseTime=true&charset=utf8mb4&loc=Local&multiStatements=true"),
		ClickHouseDSN: env("CLICKHOUSE_DSN", "clickhouse://default:audit2026@127.0.0.1:9000/audit?dial_timeout=5s&read_timeout=60s"),
		JWTSecret:     []byte(env("JWT_SECRET", "change-me-please-9f2c4e1a")),
		AdminInitPass: os.Getenv("ADMIN_INIT_PASSWORD"),
		AccessTTL:     time.Duration(atoi(env("ACCESS_TTL_SECONDS", "900"))) * time.Second,
		RefreshTTL:    time.Duration(atoi(env("REFRESH_TTL_HOURS", "168"))) * time.Hour,
		LockThreshold: atoi(env("LOCK_THRESHOLD", "5")),
		LockMinutes:   atoi(env("LOCK_MINUTES", "10")),
		RetentionDays: atoi(env("RETENTION_TTL_DAYS", "180")),
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ==================== 密码 / Token 工具 ====================

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// 等保三级密码复杂度复核（与前端 utils/password.ts 一致）
func validatePasswordStrength(pw string) error {
	if len(pw) < 8 {
		return errors.New("密码长度不得少于 8 位")
	}
	hasUpper, hasLower, hasDigit, hasSpecial := false, false, false, false
	for _, c := range pw {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("密码须同时包含大写字母、小写字母、数字与特殊字符")
	}
	return nil
}

// ==================== JWT ====================

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	CSRF     string `json:"csrf"`
	jwt.RegisteredClaims
}

func signAccess(cfg *Config, u *User, csrf string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: u.ID, Username: u.Username, Role: u.Role, CSRF: csrf,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "logaudit-api",
			Subject:   u.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.AccessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(cfg.JWTSecret)
}

func parseAccess(cfg *Config, token string) (*Claims, error) {
	claims := &Claims{}
	t, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return cfg.JWTSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("无效或过期的令牌")
	}
	return claims, nil
}

// 写请求 CSRF 校验：X-XSRF-TOKEN 必须等于 access token 中的 csrf claim
func validCSRF(claims *Claims, headerCSRF string) bool {
	return claims.CSRF != "" && headerCSRF == claims.CSRF
}

var _ = log.Printf
