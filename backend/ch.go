package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// ClickHouse 连接（audit_logs 日志流水存储）
// 环境变量 CLICKHOUSE_DSN 格式：
//   clickhouse://default:<password>@clickhouse:9000/audit?dial_timeout=5s&read_timeout=60s
func initClickHouse(cfg *Config) (clickhouse.Conn, error) {
	dsn := cfg.ClickHouseDSN
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("ClickHouse DSN 解析失败: %w", err)
	}
	opts.Settings = clickhouse.Settings{
		"max_execution_time": 60,
	}
	opts.MaxOpenConns = 5
	opts.MaxIdleConns = 2

	var conn clickhouse.Conn
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		c, err := clickhouse.Open(opts)
		if err == nil {
			if err = c.Ping(ctx); err == nil {
				conn = c
				cancel()
				break
			}
			_ = c.Close()
		}
		cancel()
		log.Printf("等待 ClickHouse 就绪（%d/30）: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if conn == nil {
		return nil, fmt.Errorf("ClickHouse 连接失败")
	}
	return conn, nil
}

// auditLogCols ClickHouse audit_logs 查询列
const auditLogCols = "ts, host, program, pid, severity, facility, source_ip, user_name, event_type, outcome, message"

// LogItem 日志条目（与前端 AuditLogItem 对齐）
type LogItem struct {
	TS            string `json:"ts"`
	Host          string `json:"host"`
	Program       string `json:"program"`
	Severity      uint8  `json:"severity"`
	SourceIP      string `json:"source_ip"`
	UserName      string `json:"user_name"`
	EventCategory string `json:"event_category"`
	EventType     string `json:"event_type"`
	Outcome       string `json:"outcome"`
	Message       string `json:"message"`
}

// eventCategory 按 event_type 推导展示类别
func eventCategory(et string) string {
	switch et {
	case "ssh_bruteforce":
		return "SSH暴力破解"
	case "login_failed":
		return "登录失败"
	case "config_change":
		return "配置变更"
	case "attack":
		return "攻击事件"
	case "auth_success":
		return "登录成功"
	case "process":
		return "进程行为"
	case "file":
		return "文件操作"
	default:
		return et
	}
}

// queryLogs 通用日志查询（参数化，防止 SQL 注入）
// conds 为 ["ts >= ?", ...]，args 为对应参数值
func (s *Server) queryLogs(ctx context.Context, conds []string, args []any, page, size int) ([]LogItem, error) {
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + joinConds(conds)
	}
	sql := "SELECT " + auditLogCols + " FROM audit.audit_logs" + where +
		" ORDER BY ts DESC LIMIT ? OFFSET ?"
	args = append(args, size, (page-1)*size)

	rows, err := s.ch.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]LogItem, 0, size)
	for rows.Next() {
		var it LogItem
		var ts time.Time
		var pid uint32
		var fac uint8
		if err := rows.Scan(&ts, &it.Host, &it.Program, &pid, &it.Severity, &fac,
			&it.SourceIP, &it.UserName, &it.EventType, &it.Outcome, &it.Message); err != nil {
			return nil, err
		}
		it.TS = ts.Format("2006-01-02 15:04:05")
		it.EventCategory = eventCategory(it.EventType)
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Server) countLogs(ctx context.Context, conds []string, args []any) (uint64, error) {
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + joinConds(conds)
	}
	var n uint64
	err := s.ch.QueryRow(ctx, "SELECT count() FROM audit.audit_logs"+where, args...).Scan(&n)
	return n, err
}

func joinConds(conds []string) string {
	out := ""
	for i, c := range conds {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
}
