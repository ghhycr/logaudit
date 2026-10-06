package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ==================== 告警规则接口 ====================
// GET /api/v1/alerts/rules          规则列表（登录可读）
// POST /api/v1/alerts/rules         admin 新增
// PUT  /api/v1/alerts/rules/{id}    admin 修改（含启停）
// DELETE /api/v1/alerts/rules/{id}  admin 删除
// POST /api/v1/alerts/rules/{id}/test  admin 试运行（dry-run，不触发动作）

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(description,''), rule_type, enabled,
		        window_seconds, COALESCE(query_filter,'{}'), threshold,
		        COALESCE(baseline_seconds,0), COALESCE(group_by,''),
		        COALESCE(realert_seconds,3600), COALESCE(severity,'warning'),
		        COALESCE(actions,'[]'), COALESCE(run_interval_seconds,60),
		        COALESCE(last_run_at,''), COALESCE(last_alert_at,''), alert_count,
		        COALESCE(created_at,''), COALESCE(updated_at,'')
		 FROM alert_rules ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "规则查询失败")
		return
	}
	defer rows.Close()

	out := make([]AlertRule, 0, 20)
	for rows.Next() {
		var ar AlertRule
		var qf, acts []byte
		var en int
		if rows.Scan(&ar.ID, &ar.Name, &ar.Description, &ar.RuleType, &en,
			&ar.WindowSeconds, &qf, &ar.Threshold, &ar.BaselineSeconds, &ar.GroupBy,
			&ar.RealertSeconds, &ar.Severity, &acts, &ar.RunIntervalSeconds,
			&ar.LastRunAt, &ar.LastAlertAt, &ar.AlertCount, &ar.CreatedAt, &ar.UpdatedAt) == nil {
			ar.Enabled = en == 1
			_ = json.Unmarshal(qf, &ar.QueryFilter)
			_ = json.Unmarshal(acts, &ar.Actions)
			out = append(out, ar)
		}
	}
	ok(w, map[string]any{"items": out, "total": len(out)})
}

// validateRule 规则字段校验（含规则类型专属字段）
func validateRule(ar *AlertRule) (string, bool) {
	ar.Name = strings.TrimSpace(ar.Name)
	if ar.Name == "" {
		return "规则名称不能为空", false
	}
	switch ar.RuleType {
	case "frequency", "spike", "flatline", "any":
	default:
		return "规则类型无效（frequency/spike/flatline/any）", false
	}
	if ar.WindowSeconds < 10 {
		ar.WindowSeconds = 300
	}
	if ar.Threshold < 1 {
		ar.Threshold = 1
	}
	if ar.RealertSeconds < 0 {
		ar.RealertSeconds = 3600
	}
	if ar.Severity == "" {
		ar.Severity = "warning"
	}
	switch ar.Severity {
	case "critical", "high", "medium", "low", "warning":
	default:
		ar.Severity = "warning"
	}
	if ar.RunIntervalSeconds < 10 {
		ar.RunIntervalSeconds = 60
	}
	return "", true
}

func (s *Server) handleCreateAlertRule(w http.ResponseWriter, r *http.Request) {
	var ar AlertRule
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	if msg, ok := validateRule(&ar); !ok {
		fail(w, http.StatusBadRequest, 4000, msg)
		return
	}
	qf, _ := json.Marshal(ar.QueryFilter)
	acts, _ := json.Marshal(ar.Actions)
	en := 0
	if ar.Enabled {
		en = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO alert_rules (name, description, rule_type, enabled, window_seconds,
		         query_filter, threshold, baseline_seconds, group_by, realert_seconds,
		         severity, actions, run_interval_seconds, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
		ar.Name, ar.Description, ar.RuleType, en, ar.WindowSeconds, qf, ar.Threshold,
		ar.BaselineSeconds, ar.GroupBy, ar.RealertSeconds, ar.Severity, acts, ar.RunIntervalSeconds)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "规则创建失败: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	ar.ID = id
	s.audit(userOf(r), "create_alert_rule", ar.Name, clientIP(r), "新增告警规则 "+ar.Name)
	ok(w, ar)
}

func (s *Server) handleUpdateAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "规则 ID 无效")
		return
	}
	var ar AlertRule
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	if msg, ok := validateRule(&ar); !ok {
		fail(w, http.StatusBadRequest, 4000, msg)
		return
	}
	qf, _ := json.Marshal(ar.QueryFilter)
	acts, _ := json.Marshal(ar.Actions)
	en := 0
	if ar.Enabled {
		en = 1
	}
	if _, err := s.db.Exec(
		`UPDATE alert_rules SET name=?, description=?, rule_type=?, enabled=?,
		         window_seconds=?, query_filter=?, threshold=?, baseline_seconds=?,
		         group_by=?, realert_seconds=?, severity=?, actions=?,
		         run_interval_seconds=?, updated_at=NOW() WHERE id=?`,
		ar.Name, ar.Description, ar.RuleType, en, ar.WindowSeconds, qf, ar.Threshold,
		ar.BaselineSeconds, ar.GroupBy, ar.RealertSeconds, ar.Severity, acts,
		ar.RunIntervalSeconds, id); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "规则更新失败: "+err.Error())
		return
	}
	ar.ID = id
	s.audit(userOf(r), "update_alert_rule", ar.Name, clientIP(r), "修改告警规则 "+ar.Name)
	ok(w, ar)
}

func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "规则 ID 无效")
		return
	}
	if _, err := s.db.Exec("DELETE FROM alert_rules WHERE id=?", id); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "规则删除失败: "+err.Error())
		return
	}
	s.audit(userOf(r), "delete_alert_rule", strconv.FormatInt(id, 10), clientIP(r), "删除告警规则 #"+strconv.FormatInt(id, 10))
	ok(w, nil)
}

// handleTestAlertRule 试运行：立即执行一次规则，返回命中结果但不触发告警动作
func (s *Server) handleTestAlertRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "规则 ID 无效")
		return
	}
	var ar AlertRule
	var qf, acts []byte
	var en int
	if err := s.db.QueryRow(
		`SELECT id, name, COALESCE(description,''), rule_type, enabled,
		        window_seconds, COALESCE(query_filter,'{}'), threshold,
		        COALESCE(baseline_seconds,0), COALESCE(group_by,''),
		        COALESCE(realert_seconds,3600), COALESCE(severity,'warning'),
		        COALESCE(actions,'[]'), COALESCE(run_interval_seconds,60)
		 FROM alert_rules WHERE id=?`, id).Scan(
		&ar.ID, &ar.Name, &ar.Description, &ar.RuleType, &en, &ar.WindowSeconds, &qf,
		&ar.Threshold, &ar.BaselineSeconds, &ar.GroupBy, &ar.RealertSeconds,
		&ar.Severity, &acts, &ar.RunIntervalSeconds); err != nil {
		fail(w, http.StatusNotFound, 4040, "规则不存在")
		return
	}
	ar.Enabled = en == 1
	_ = json.Unmarshal(qf, &ar.QueryFilter)
	_ = json.Unmarshal(acts, &ar.Actions)

	ctx, cancel := contextTimeout(r, 30)
	defer cancel()
	now := time.Now()
	window := time.Duration(ar.WindowSeconds) * time.Second
	conds, args := alertFilterConds(ar.QueryFilter)
	groupBy := strings.TrimSpace(ar.GroupBy)
	switch groupBy {
	case "host", "source_ip", "event_type", "user_name":
	default:
		groupBy = ""
	}

	runArgs := append([]any{}, args...)
	runArgs = append(runArgs, now.Add(-window))
	q := "SELECT count() AS c"
	if groupBy != "" {
		q += ", " + groupBy
	}
	q += " FROM audit.audit_logs WHERE " + joinConds(append(append([]string{}, conds...), "ts >= ?"))
	if groupBy != "" {
		q += " GROUP BY " + groupBy
	}

	matches := map[string]int64{}
	if groupBy == "" {
		var n uint64
		if err := s.ch.QueryRow(ctx, q, runArgs...).Scan(&n); err == nil {
			matches["全部"] = int64(n)
		}
	} else {
		rows, err := s.ch.Query(ctx, q, runArgs...)
		if err == nil {
			for rows.Next() {
				var c uint64
				var k string
				if rows.Scan(&c, &k) == nil {
					matches[k] = int64(c)
				}
			}
			rows.Close()
		}
	}

	ok(w, map[string]any{
		"rule_id":   id,
		"rule_name": ar.Name,
		"window_seconds": ar.WindowSeconds,
		"matched":   matches,
		"dry_run":   true,
	})
}

// ==================== 告警事件接口 ====================
// GET /api/v1/alerts/events?status=&rule_id=&page=&size=
// POST /api/v1/alerts/events/{id}/ack
// GET /api/v1/alerts/events/stats

func (s *Server) handleListAlertEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	size := atoiDefault(q.Get("size"), 20)
	if size < 1 || size > 100 {
		size = 20
	}
	conds := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if st := strings.TrimSpace(q.Get("status")); st != "" {
		conds = append(conds, "status=?")
		args = append(args, st)
	}
	if rid := strings.TrimSpace(q.Get("rule_id")); rid != "" {
		conds = append(conds, "rule_id=?")
		args = append(args, rid)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM alert_events"+where, args...).Scan(&total)

	rows, err := s.db.Query(
		"SELECT id, rule_id, rule_name, severity, match_key, match_count, message, status, COALESCE(acked_by,''), created_at"+
			" FROM alert_events"+where+" ORDER BY id DESC LIMIT ? OFFSET ?",
		append(append([]any{}, args...), size, (page-1)*size)...)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "告警事件查询失败")
		return
	}
	defer rows.Close()

	items := make([]AlertEvent, 0, size)
	for rows.Next() {
		var ae AlertEvent
		if rows.Scan(&ae.ID, &ae.RuleID, &ae.RuleName, &ae.Severity, &ae.MatchKey,
			&ae.MatchCount, &ae.Message, &ae.Status, &ae.AckedBy, &ae.CreatedAt) == nil {
			items = append(items, ae)
		}
	}
	ok(w, map[string]any{"items": items, "total": total, "page": page, "size": size})
}

func (s *Server) handleAckAlertEvent(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "事件 ID 无效")
		return
	}
	if _, err := s.db.Exec(
		"UPDATE alert_events SET status='acked', acked_by=? WHERE id=? AND status='open'",
		userOf(r), id); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "确认失败")
		return
	}
	s.audit(userOf(r), "ack_alert", strconv.FormatInt(id, 10), clientIP(r), "确认告警事件 #"+strconv.FormatInt(id, 10))
	ok(w, nil)
}

func (s *Server) handleAlertStats(w http.ResponseWriter, r *http.Request) {
	var open, today, total int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM alert_events WHERE status='open'").Scan(&open)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM alert_events WHERE created_at >= CURDATE()").Scan(&today)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM alert_events").Scan(&total)
	ok(w, map[string]any{"open": open, "today": today, "total": total})
}

// contextTimeout 便捷超时上下文
func contextTimeout(r *http.Request, sec int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), time.Duration(sec)*time.Second)
}
