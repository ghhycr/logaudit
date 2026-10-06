package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
)

// Server 持有全局依赖
type Server struct {
	cfg *Config
	db  *sql.DB
	ch  clickhouse.Conn
	rdb *redis.Client // 可选：登录防爆破 / 告警去重 / 统计缓存（nil 时降级 MySQL）
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	cfg := loadConfig()

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("MySQL 初始化失败: %v", err)
	}
	defer db.Close()

	ch, err := initClickHouse(cfg)
	if err != nil {
		log.Fatalf("ClickHouse 初始化失败: %v", err)
	}
	defer ch.Close()

	rdb := initRedis() // 可选，失败降级
	if rdb != nil {
		defer rdb.Close()
	}

	if pw, err := ensureAdmin(cfg, db); err != nil {
		log.Fatalf("初始化 admin 用户失败: %v", err)
	} else if pw != "" {
		log.Printf("==============================================")
		log.Printf("已创建初始管理员账号 admin")
		log.Printf("初始密码: %s（请登录后立即修改）", pw)
		log.Printf("==============================================")
	}

	s := &Server{cfg: cfg, db: db, ch: ch, rdb: rdb}
	mux := http.NewServeMux()

	// ---- M2 认证 ----
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.handleRefresh)
	mux.HandleFunc("POST /api/v1/auth/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /api/v1/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("POST /api/v1/auth/change-password", s.requireAuth(s.handleChangePassword))

	// ---- M3 日志检索 / 统计 / 设备 / 留存 / 用户 ----
	mux.HandleFunc("GET /api/v1/logs/search", s.requireAuth(s.handleSearchLogs))
	mux.HandleFunc("GET /api/v1/logs/{key}", s.requireAuth(s.handleLogDetail))
	mux.HandleFunc("POST /api/v1/logs/export", s.requireAuth(s.handleExportLogs))
	mux.HandleFunc("GET /api/v1/stats/overview", s.requireAuth(s.handleStatsOverview))
	mux.HandleFunc("GET /api/v1/stats/login-fail", s.requireAuth(s.handleLoginFailStats))
	mux.HandleFunc("GET /api/v1/devices", s.requireAuth(s.handleListDevices))
	mux.HandleFunc("POST /api/v1/devices", s.requireAuth(s.requireRole("admin", s.handleCreateDevice)))
	mux.HandleFunc("PUT /api/v1/devices/{id}", s.requireAuth(s.requireRole("admin", s.handleUpdateDevice)))
	mux.HandleFunc("DELETE /api/v1/devices/{id}", s.requireAuth(s.requireRole("admin", s.handleDeleteDevice)))
	mux.HandleFunc("GET /api/v1/retention", s.requireAuth(s.handleRetention))
	mux.HandleFunc("GET /api/v1/users", s.requireAuth(s.requireRole("admin", s.handleListUsers)))
	mux.HandleFunc("POST /api/v1/users", s.requireAuth(s.requireRole("admin", s.handleCreateUser)))
	mux.HandleFunc("PUT /api/v1/users/{id}", s.requireAuth(s.requireRole("admin", s.handleUpdateUser)))
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.requireAuth(s.requireRole("admin", s.handleDeleteUser)))
	mux.HandleFunc("POST /api/v1/users/{id}/reset-password", s.requireAuth(s.requireRole("admin", s.handleResetPassword)))
	mux.HandleFunc("GET /api/v1/settings/ntp", s.requireAuth(s.requireRole("admin", s.handleGetNTP)))
	mux.HandleFunc("PUT /api/v1/settings/ntp", s.requireAuth(s.requireRole("admin", s.handlePutNTP)))
	// ---- M5 告警规则与事件 ----
	mux.HandleFunc("GET /api/v1/alerts/rules", s.requireAuth(s.handleListAlertRules))
	mux.HandleFunc("POST /api/v1/alerts/rules", s.requireAuth(s.requireRole("admin", s.handleCreateAlertRule)))
	mux.HandleFunc("PUT /api/v1/alerts/rules/{id}", s.requireAuth(s.requireRole("admin", s.handleUpdateAlertRule)))
	mux.HandleFunc("DELETE /api/v1/alerts/rules/{id}", s.requireAuth(s.requireRole("admin", s.handleDeleteAlertRule)))
	mux.HandleFunc("POST /api/v1/alerts/rules/{id}/test", s.requireAuth(s.requireRole("admin", s.handleTestAlertRule)))
	mux.HandleFunc("GET /api/v1/alerts/events", s.requireAuth(s.handleListAlertEvents))
	mux.HandleFunc("POST /api/v1/alerts/events/{id}/ack", s.requireAuth(s.handleAckAlertEvent))
	mux.HandleFunc("GET /api/v1/alerts/events/stats", s.requireAuth(s.handleAlertStats))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"status": "up"})
	})

	// 启动 M5 告警引擎（随 API 常驻，规则变更即时生效）
	engine := newAlertEngine(s)
	engine.Start()

	addr := ":" + cfg.Port
	log.Printf("LogAudit API 启动，监听 %s（M2 认证 + M3 检索 + M5 告警）", addr)
	if err := http.ListenAndServe(addr, recoverPanic(securityHeaders(mux))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
