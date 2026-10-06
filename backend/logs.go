package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ==================== 日志检索 ====================
// GET /api/v1/logs/search?from=&to=&host=&source_ip=&event_type=&q=&page=&size=

func (s *Server) handleSearchLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	size := atoiDefault(q.Get("size"), 20)
	if size < 1 || size > 500 {
		size = 20
	}
	if page < 1 {
		page = 1
	}

	conds := make([]string, 0, 6)
	args := make([]any, 0, 6)

	if from := strings.TrimSpace(q.Get("from")); from != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", from); err == nil {
			conds = append(conds, "ts >= ?")
			args = append(args, t)
		} else if t, err := time.Parse(time.RFC3339, from); err == nil {
			conds = append(conds, "ts >= ?")
			args = append(args, t)
		}
	}
	if to := strings.TrimSpace(q.Get("to")); to != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", to); err == nil {
			conds = append(conds, "ts <= ?")
			args = append(args, t)
		} else if t, err := time.Parse(time.RFC3339, to); err == nil {
			conds = append(conds, "ts <= ?")
			args = append(args, t)
		}
	}
	if host := strings.TrimSpace(q.Get("host")); host != "" {
		conds = append(conds, "host = ?")
		args = append(args, host)
	}
	if sip := strings.TrimSpace(q.Get("source_ip")); sip != "" {
		conds = append(conds, "source_ip = ?")
		args = append(args, sip)
	}
	if et := strings.TrimSpace(q.Get("event_type")); et != "" {
		conds = append(conds, "event_type = ?")
		args = append(args, et)
	}
	if kw := strings.TrimSpace(q.Get("q")); kw != "" {
		esc := strings.NewReplacer(`%`, `\%`, `_`, `\_`).Replace(kw)
		like := "%" + esc + "%"
		conds = append(conds, "(message ILIKE ? OR raw ILIKE ?)")
		args = append(args, like, like)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	total, err := s.countLogs(ctx, conds, args)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "日志计数查询失败: "+err.Error())
		return
	}
	items, err := s.queryLogs(ctx, conds, args, page, size)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "日志检索失败: "+err.Error())
		return
	}
	ok(w, map[string]any{
		"total": total, "page": page, "size": size, "items": items,
	})
}

// ==================== 日志详情 ====================
// GET /api/v1/logs/{ts}|{host} —— 前端当前以行数据直取详情，此接口按 ts+host 回查单条
func (s *Server) handleLogDetail(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		fail(w, http.StatusBadRequest, 4000, "日志详情参数格式错误")
		return
	}
	ts, err := time.Parse("2006-01-02 15:04:05", parts[0])
	if err != nil {
		fail(w, http.StatusBadRequest, 4000, "时间参数格式错误")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := s.ch.Query(ctx,
		"SELECT "+auditLogCols+" FROM audit.audit_logs WHERE ts = ? AND host = ? LIMIT 1",
		ts, parts[1])
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "查询失败")
		return
	}
	defer rows.Close()

	if !rows.Next() {
		fail(w, http.StatusNotFound, 4040, "日志不存在")
		return
	}
	var it LogItem
	var t time.Time
	var pid uint32
	var fac uint8
	if err := rows.Scan(&t, &it.Host, &it.Program, &pid, &it.Severity, &fac,
		&it.SourceIP, &it.UserName, &it.EventType, &it.Outcome, &it.Message); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "解析失败")
		return
	}
	it.TS = t.Format("2006-01-02 15:04:05")
	it.EventCategory = eventCategory(it.EventType)
	ok(w, it)
}

// ==================== CSV 导出（异步任务占位，M4 完善） ====================
// POST /api/v1/logs/export {from,to,q,...} -> {task_id}
func (s *Server) handleExportLogs(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	taskID := hex.EncodeToString(b)
	s.audit(userOf(r), "export_logs", "audit_logs", clientIP(r), "发起日志导出任务")
	ok(w, map[string]any{"task_id": taskID})
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return def
	}
	return n
}

func userOf(r *http.Request) string {
	if v, ok := r.Context().Value(ctxUser).(string); ok {
		return v
	}
	return ""
}
