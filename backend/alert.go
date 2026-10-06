package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ==================== M5 自研告警引擎（等价 ElastAlert2 核心能力） ====================
// 规则类型：
//   frequency — 窗口内事件数 >= 阈值
//   spike     — 当前窗口事件数 >= 基线窗口 * 阈值（突增）
//   flatline  — 窗口内事件数 < 阈值（设备静默）
//   any       — 窗口内存在匹配事件即告警
// 去重：realert 窗口内同一 (rule, match_key) 只告警一次
// 动作：webhook（HTTP POST JSON）/ SMTP / 平台内 alert_events 记录（始终）

// AlertRule 告警规则
type AlertRule struct {
	ID                int64        `json:"id"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	RuleType          string       `json:"rule_type"`
	Enabled           bool         `json:"enabled"`
	WindowSeconds     int          `json:"window_seconds"`
	QueryFilter       QueryFilter  `json:"query_filter"`
	Threshold         int          `json:"threshold"`
	BaselineSeconds   int          `json:"baseline_seconds"`
	GroupBy           string       `json:"group_by"`
	RealertSeconds    int          `json:"realert_seconds"`
	Severity          string       `json:"severity"`
	Actions           []AlertAction `json:"actions"`
	RunIntervalSeconds int        `json:"run_interval_seconds"`
	LastRunAt         string       `json:"last_run_at"`
	LastAlertAt       string       `json:"last_alert_at"`
	AlertCount        int64        `json:"alert_count"`
	CreatedAt         string       `json:"created_at"`
	UpdatedAt         string       `json:"updated_at"`
}

// QueryFilter 日志匹配条件
type QueryFilter struct {
	Host        string `json:"host"`
	SourceIP    string `json:"source_ip"`
	EventType   string `json:"event_type"`
	Keyword     string `json:"keyword"`
	MinSeverity uint8  `json:"min_severity"`
}

// AlertAction 告警动作
type AlertAction struct {
	Type    string            `json:"type"` // webhook | smtp
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	To      []string          `json:"to"`
}

// AlertEvent 告警事件（平台内记录 + 处置）
type AlertEvent struct {
	ID         int64  `json:"id"`
	RuleID     int64  `json:"rule_id"`
	RuleName   string `json:"rule_name"`
	Severity   string `json:"severity"`
	MatchKey   string `json:"match_key"`
	MatchCount int64  `json:"match_count"`
	Message    string `json:"message"`
	Hits       []LogItem `json:"hits,omitempty"`
	Status     string `json:"status"` // open | acked | closed
	AckedBy    string `json:"acked_by"`
	CreatedAt  string `json:"created_at"`
}

// AlertEngine 告警引擎调度器
type AlertEngine struct {
	srv    *Server
	stop   chan struct{}
	done   chan struct{}
	mu     sync.Mutex
	active bool
}

func newAlertEngine(s *Server) *AlertEngine {
	return &AlertEngine{srv: s, stop: make(chan struct{}), done: make(chan struct{})}
}

// Start 启动调度器：每 10 秒扫描一次启用规则，按 run_interval_seconds 节流
func (e *AlertEngine) Start() {
	e.mu.Lock()
	if e.active {
		e.mu.Unlock()
		return
	}
	e.active = true
	e.mu.Unlock()

	go func() {
		defer close(e.done)
		log.Printf("告警引擎已启动（调度周期 10s）")
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-e.stop:
				log.Printf("告警引擎已停止")
				return
			case <-ticker.C:
				e.srv.evaluateAllRules()
			}
		}
	}()
}

func (e *AlertEngine) Stop() {
	e.mu.Lock()
	if !e.active {
		e.mu.Unlock()
		return
	}
	e.active = false
	e.mu.Unlock()
	close(e.stop)
	<-e.done
}

// loadEnabledRules 从 MySQL 加载启用规则（规则变更即时生效）
func (s *Server) loadEnabledRules() ([]AlertRule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(description,''), rule_type, enabled,
		        window_seconds, COALESCE(query_filter,'{}'), threshold,
		        COALESCE(baseline_seconds,0), COALESCE(group_by,''),
		        COALESCE(realert_seconds,3600), COALESCE(severity,'warning'),
		        COALESCE(actions,'[]'), COALESCE(run_interval_seconds,60),
		        COALESCE(last_run_at,''), COALESCE(last_alert_at,''), alert_count,
		        COALESCE(created_at,'')
		 FROM alert_rules WHERE enabled=1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AlertRule, 0, 20)
	for rows.Next() {
		var r AlertRule
		var qf, acts []byte
		var en int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.RuleType, &en,
			&r.WindowSeconds, &qf, &r.Threshold, &r.BaselineSeconds, &r.GroupBy,
			&r.RealertSeconds, &r.Severity, &acts, &r.RunIntervalSeconds,
			&r.LastRunAt, &r.LastAlertAt, &r.AlertCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = en == 1
		_ = json.Unmarshal(qf, &r.QueryFilter)
		_ = json.Unmarshal(acts, &r.Actions)
		out = append(out, r)
	}
	return out, rows.Err()
}

// evaluateAllRules 扫描所有启用规则（单规则失败不影响其他）
func (s *Server) evaluateAllRules() {
	rules, err := s.loadEnabledRules()
	if err != nil {
		log.Printf("[alert] 加载规则失败: %v", err)
		return
	}
	for _, r := range rules {
		if err := s.evaluateRule(&r); err != nil {
			log.Printf("[alert] 规则 %d(%s) 执行失败: %v", r.ID, r.Name, err)
		}
	}
}

// evaluateRule 执行单条规则
func (s *Server) evaluateRule(r *AlertRule) error {
	interval := time.Duration(r.RunIntervalSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	if r.LastRunAt != "" {
		last, err := time.Parse("2006-01-02 15:04:05", r.LastRunAt)
		if err == nil && time.Since(last) < interval {
			return nil // 未到运行间隔
		}
	}

	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	window := time.Duration(r.WindowSeconds) * time.Second
	if window < 10*time.Second {
		window = 5 * time.Minute
	}

	// 构建过滤条件
	conds, args := alertFilterConds(r.QueryFilter)

	var matches map[string]int64
	var sampleHits []LogItem

	switch r.RuleType {
	case "flatline":
		// 窗口内无日志（或 < 阈值）即告警
		total, err := s.countLogs(ctx, conds, args)
		if err != nil {
			return err
		}
		th := r.Threshold
		if th < 1 {
			th = 1
		}
		if total >= uint64(th) {
			return s.markRuleRun(r)
		}
		matches = map[string]int64{"": 1}

	case "spike":
		// 当前窗口 vs 基线窗口（紧邻之前 baseline_seconds 窗口）
		base := time.Duration(r.BaselineSeconds) * time.Second
		if base < time.Minute {
			base = time.Hour
		}
		curArgs := append([]any{}, args...)
		curConds := append([]string{}, conds...)
		curConds = append(curConds, "ts >= ? AND ts <= ?")
		curArgs = append(curArgs, now.Add(-window), now)
		curTotal, err := s.countLogs(ctx, curConds, curArgs)
		if err != nil {
			return err
		}
		baseConds := append([]string{}, conds...)
		baseArgs := append([]any{}, args...)
		baseConds = append(baseConds, "ts >= ? AND ts <= ?")
		baseArgs = append(baseArgs, now.Add(-window-base), now.Add(-base))
		baseTotal, err := s.countLogs(ctx, baseConds, baseArgs)
		if err != nil {
			return err
		}
		mult := r.Threshold
		if mult < 2 {
			mult = 2
		}
		if baseTotal > 0 && curTotal >= uint64(baseTotal)*uint64(mult) {
			matches = map[string]int64{"spike": int64(curTotal)}
		}
		if baseTotal == 0 && curTotal >= uint64(mult) {
			matches = map[string]int64{"spike": int64(curTotal)}
		}

	default: // frequency / any
		matches = make(map[string]int64)
		groupBy := strings.TrimSpace(r.GroupBy)
		// 仅允许白名单聚合维度，非法值视为不分组
		switch groupBy {
		case "host", "source_ip", "event_type", "user_name":
		default:
			groupBy = ""
		}
		th := r.Threshold
		if th < 1 {
			th = 1
		}
		var q string
		runArgs := append([]any{}, args...)
		runArgs = append(runArgs, now.Add(-window))
		if groupBy == "" {
			q = "SELECT count() FROM audit.audit_logs WHERE " + joinConds(append(append([]string{}, conds...), "ts >= ?"))
			var n uint64
			if err := s.ch.QueryRow(ctx, q, runArgs...).Scan(&n); err != nil {
				return err
			}
			if n >= uint64(th) {
				matches[""] = int64(n)
			}
		} else {
			q = "SELECT count() AS c, " + groupBy + " FROM audit.audit_logs WHERE " +
				joinConds(append(append([]string{}, conds...), "ts >= ?")) + " GROUP BY " + groupBy
			rows, err := s.ch.Query(ctx, q, runArgs...)
			if err != nil {
				return err
			}
			for rows.Next() {
				var c uint64
				var key string
				if rows.Scan(&c, &key) == nil && c >= uint64(th) {
					matches[key] = int64(c)
				}
			}
			rows.Close()
		}
	}

	// 命中样本（用于告警内容）
	if len(matches) > 0 {
		sampConds := append([]string{}, conds...)
		sampArgs := append([]any{}, args...)
		sampConds = append(sampConds, "ts >= ?")
		sampArgs = append(sampArgs, now.Add(-window))
		sampleHits, _ = s.queryLogs(ctx, sampConds, sampArgs, 1, 10)
	}

	triggered := false
	for key, cnt := range matches {
		// realert 去重：同规则+key 在 realert 窗口内已告警则跳过
		if r.RealertSeconds > 0 && s.alertRecent(r.ID, key, r.RealertSeconds) {
			continue
		}
		if err := s.fireAlert(r, key, cnt, sampleHits); err != nil {
			log.Printf("[alert] 规则 %d 触发告警失败: %v", r.ID, err)
			continue
		}
		triggered = true
	}

	if err := s.markRuleRun(r); err != nil {
		return err
	}
	if triggered {
		_ = s.markRuleAlert(r)
	}
	return nil
}

// alertFilterConds 组装 ClickHouse 过滤条件（与检索同源，参数化防注入）
func alertFilterConds(f QueryFilter) ([]string, []any) {
	conds := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if f.Host != "" {
		conds = append(conds, "host = ?")
		args = append(args, f.Host)
	}
	if f.SourceIP != "" {
		conds = append(conds, "source_ip = ?")
		args = append(args, f.SourceIP)
	}
	if f.EventType != "" {
		conds = append(conds, "event_type = ?")
		args = append(args, f.EventType)
	}
	if f.Keyword != "" {
		esc := strings.NewReplacer(`%`, `\%`, `_`, `\_`).Replace(f.Keyword)
		like := "%" + esc + "%"
		conds = append(conds, "(message ILIKE ? OR raw ILIKE ?)")
		args = append(args, like, like)
	}
	if f.MinSeverity > 0 {
		conds = append(conds, "severity <= ?")
		args = append(args, f.MinSeverity)
	}
	return conds, args
}

// alertRecent realert 去重检查：同规则+key 在 realert 窗口内是否已产生告警
// Redis 优先（SET NX + EXPIRE 原子去重）；未接入 Redis 时回退 MySQL 窗口查询
func (s *Server) alertRecent(ruleID int64, key string, windowSec int) bool {
	if s.rdb != nil {
		ctx := context.Background()
		rkey := fmt.Sprintf("alert:realert:%d:%s", ruleID, key)
		// NX 成功 = 窗口内首次触发（放行并标记）；失败 = 窗口内已告警（去重）
		ok, err := s.rdb.SetNX(ctx, rkey, "1", time.Duration(windowSec)*time.Second).Result()
		if err != nil {
			log.Printf("[alert] Redis 去重检查失败（%v），回退 MySQL", err)
		} else {
			return !ok
		}
	}
	// MySQL 回退：窗口内存在告警事件即去重
	var n int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM alert_events
		 WHERE rule_id=? AND match_key=? AND created_at >= DATE_SUB(NOW(), INTERVAL `+strconv.Itoa(windowSec)+` SECOND)`,
		ruleID, key).Scan(&n)
	return n > 0
}

// fireAlert 触发告警：写平台记录 + 执行动作
func (s *Server) fireAlert(r *AlertRule, key string, cnt int64, hits []LogItem) error {
	keyDesc := key
	if key == "" {
		keyDesc = "全部"
	}
	msg := fmt.Sprintf("告警规则「%s」触发：%s 在 %d 秒窗口内命中 %d 条（%s）",
		r.Name, keyDesc, r.WindowSeconds, cnt, eventCategory(r.QueryFilter.EventType))

	res, err := s.db.Exec(
		`INSERT INTO alert_events (rule_id, rule_name, severity, match_key, match_count, message, status, created_at)
		 VALUES (?,?,?,?,?,?,'open',NOW())`,
		r.ID, r.Name, r.Severity, key, cnt, msg)
	if err != nil {
		return fmt.Errorf("写入告警事件失败: %w", err)
	}
	evID, _ := res.LastInsertId()

	log.Printf("[alert] 触发告警 rule=%d(%s) key=%s count=%d", r.ID, r.Name, key, cnt)

	// 执行动作
	for _, a := range r.Actions {
		switch a.Type {
		case "webhook":
			if err := sendWebhook(a, r, key, cnt, hits, evID); err != nil {
				log.Printf("[alert] webhook 发送失败: %v", err)
			}
		case "smtp":
			if err := sendAlertMail(a, r, key, cnt); err != nil {
				log.Printf("[alert] SMTP 发送失败: %v", err)
			}
		}
	}
	return nil
}

// sendWebhook HTTP POST JSON 到告警接收端
func sendWebhook(a AlertAction, r *AlertRule, key string, cnt int64, hits []LogItem, evID int64) error {
	payload := map[string]any{
		"event_id":    evID,
		"rule_id":     r.ID,
		"rule_name":   r.Name,
		"severity":    r.Severity,
		"match_key":   key,
		"match_count": cnt,
		"window_seconds": r.WindowSeconds,
		"message":     fmt.Sprintf("规则「%s」命中 %d 条（%s）", r.Name, cnt, key),
		"hits":        hits,
		"ts":          time.Now().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, a.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range a.Headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 %d", resp.StatusCode)
	}
	return nil
}

// sendAlertMail 通过 SMTP 发送告警邮件（需全局 SMTP 配置）
func sendAlertMail(a AlertAction, r *AlertRule, key string, cnt int64) error {
	host := env("SMTP_HOST", "")
	port := env("SMTP_PORT", "25")
	from := env("SMTP_FROM", "")
	user := env("SMTP_USER", "")
	pass := env("SMTP_PASSWORD", "")
	if host == "" || from == "" || len(a.To) == 0 {
		return fmt.Errorf("SMTP 未配置或收件人为空")
	}
	subject := fmt.Sprintf("[%s] 日志审计告警：%s", strings.ToUpper(r.Severity), r.Name)
	body := fmt.Sprintf("告警规则：%s\n级别：%s\n匹配维度：%s\n命中数量：%d\n窗口：%d 秒\n时间：%s\n",
		r.Name, r.Severity, key, cnt, r.WindowSeconds, time.Now().Format("2006-01-02 15:04:05"))
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, strings.Join(a.To, ","), subject, body)

	addr := fmt.Sprintf("%s:%s", host, port)
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return smtp.SendMail(addr, auth, from, a.To, []byte(msg))
}

// markRuleRun 更新 last_run_at
func (s *Server) markRuleRun(r *AlertRule) error {
	_, err := s.db.Exec("UPDATE alert_rules SET last_run_at=NOW() WHERE id=?", r.ID)
	return err
}

// markRuleAlert 更新 last_alert_at 与告警计数
func (s *Server) markRuleAlert(r *AlertRule) error {
	_, err := s.db.Exec(
		"UPDATE alert_rules SET last_alert_at=NOW(), alert_count=alert_count+1 WHERE id=?", r.ID)
	return err
}
