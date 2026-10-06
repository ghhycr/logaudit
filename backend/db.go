package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// ==================== 模型 ====================

type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	DisplayName  string     `json:"display_name"`
	Role         string     `json:"role"`
	Status       int        `json:"-"` // 1 正常 0 停用
	FailCount    int        `json:"-"`
	LockUntil    *time.Time `json:"-"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// ==================== 初始化 ====================

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
  id           BIGINT AUTO_INCREMENT PRIMARY KEY,
  username     VARCHAR(64)  NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(128) NOT NULL DEFAULT '',
  role         VARCHAR(16)  NOT NULL DEFAULT 'viewer',
  status       TINYINT      NOT NULL DEFAULT 1,
  fail_count   INT          NOT NULL DEFAULT 0,
  lock_until   DATETIME     NULL,
  last_login_at DATETIME    NULL,
  created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_role (role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS refresh_tokens (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id    BIGINT       NOT NULL,
  token_hash CHAR(64)     NOT NULL UNIQUE, -- SHA-256(refresh_token)
  expires_at DATETIME     NOT NULL,
  revoked    TINYINT      NOT NULL DEFAULT 0,
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS op_audits (
  id       BIGINT AUTO_INCREMENT PRIMARY KEY,
  username VARCHAR(64)  NOT NULL DEFAULT '',
  action   VARCHAR(64)  NOT NULL,
  target   VARCHAR(255) NOT NULL DEFAULT '',
  detail   VARCHAR(512) NOT NULL DEFAULT '',
  ip       VARCHAR(64)  NOT NULL DEFAULT '',
  ts       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_ts (ts),
  KEY idx_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS devices (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(128) NOT NULL,
  ip         VARCHAR(64)  NOT NULL,
  vendor     VARCHAR(64)  NOT NULL DEFAULT '',
  model      VARCHAR(64)  NOT NULL DEFAULT '',
  kind       VARCHAR(32)  NOT NULL DEFAULT '',
  location   VARCHAR(128) NOT NULL DEFAULT '',
  enabled    TINYINT      NOT NULL DEFAULT 1,
  updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_ip (ip)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS alert_rules (
  id                  BIGINT AUTO_INCREMENT PRIMARY KEY,
  name                VARCHAR(128) NOT NULL,
  description         VARCHAR(512) NOT NULL DEFAULT '',
  rule_type           VARCHAR(32)  NOT NULL,
  enabled             TINYINT      NOT NULL DEFAULT 1,
  window_seconds      INT          NOT NULL DEFAULT 300,
  query_filter        JSON         NOT NULL,
  threshold           INT          NOT NULL DEFAULT 5,
  baseline_seconds    INT          NOT NULL DEFAULT 3600,
  group_by            VARCHAR(64)  NOT NULL DEFAULT '',
  realert_seconds     INT          NOT NULL DEFAULT 3600,
  severity            VARCHAR(16)  NOT NULL DEFAULT 'warning',
  actions             JSON         NOT NULL,
  run_interval_seconds INT         NOT NULL DEFAULT 60,
  last_run_at         DATETIME     NULL,
  last_alert_at       DATETIME     NULL,
  alert_count         BIGINT       NOT NULL DEFAULT 0,
  created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_enabled (enabled),
  KEY idx_type (rule_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS alert_events (
  id          BIGINT AUTO_INCREMENT PRIMARY KEY,
  rule_id     BIGINT       NOT NULL,
  rule_name   VARCHAR(128) NOT NULL,
  severity    VARCHAR(16)  NOT NULL DEFAULT 'warning',
  match_key   VARCHAR(255) NOT NULL DEFAULT '',
  match_count BIGINT       NOT NULL DEFAULT 0,
  message     TEXT,
  status      VARCHAR(16)  NOT NULL DEFAULT 'open',
  acked_by    VARCHAR(64)  NOT NULL DEFAULT '',
  created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_rule (rule_id),
  KEY idx_status (status),
  KEY idx_ts (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

func initDB(cfg *Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// MySQL 容器启动需要时间，重试连接
	var pingErr error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pingErr = db.PingContext(ctx)
		cancel()
		if pingErr == nil {
			break
		}
		log.Printf("等待 MySQL 就绪（%d/30）: %v", i+1, pingErr)
		time.Sleep(2 * time.Second)
	}
	if pingErr != nil {
		return nil, fmt.Errorf("MySQL 连接失败: %w", pingErr)
	}

	for _, stmt := range []string{schemaSQL} {
		if _, err := db.Exec(stmt); err != nil {
			return nil, fmt.Errorf("执行建表失败: %w", err)
		}
	}
	return db, nil
}

// ensureAdmin 首次启动时创建初始 admin 用户
// 密码优先级：ADMIN_INIT_PASSWORD 环境变量 > 随机生成（打印到日志）
func ensureAdmin(cfg *Config, db *sql.DB) (string, error) {
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username='admin'").Scan(&cnt); err != nil {
		return "", err
	}
	if cnt > 0 {
		return "", nil
	}

	pw := cfg.AdminInitPass
	if pw == "" {
		rand, err := randomHex(12)
		if err != nil {
			return "", err
		}
		pw = "Aa1!" + rand // 满足复杂度：大小写数字特殊
	}
	hash, err := hashPassword(pw)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(
		"INSERT INTO users (username, password_hash, display_name, role, status) VALUES (?,?,?,?,1)",
		"admin", hash, "系统管理员", "admin",
	); err != nil {
		return "", err
	}
	return pw, nil
}

// ==================== 查询 ====================

func getUserByName(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, password_hash, display_name, role, status, fail_count, lock_until, last_login_at
		 FROM users WHERE username=?`, username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.FailCount, &u.LockUntil, &u.LastLoginAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func getUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, password_hash, display_name, role, status, fail_count, lock_until, last_login_at
		 FROM users WHERE id=?`, id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.Status, &u.FailCount, &u.LockUntil, &u.LastLoginAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (u *User) public() map[string]any {
	m := map[string]any{
		"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "role": u.Role,
		"locked": u.LockUntil != nil && u.LockUntil.After(time.Now()),
	}
	if u.LastLoginAt != nil {
		m["last_login_at"] = u.LastLoginAt.Format("2006-01-02 15:04:05")
	}
	return m
}
