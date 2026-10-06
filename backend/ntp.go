package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ==================== NTP 服务器配置（系统设置 · NTP，仅 admin） ====================
// GET /api/v1/settings/ntp   读取配置（settings 表 ntp_config，缺省给默认值）
// PUT /api/v1/settings/ntp   保存配置

type NtpConfig struct {
	Server1       string `json:"server1"`
	Server2       string `json:"server2"`
	IntervalHours int    `json:"interval_hours"` // 同步间隔（小时），1-168
	Enabled       bool   `json:"enabled"`
}

func defaultNTP() NtpConfig {
	return NtpConfig{
		Server1:       "ntp.aliyun.com",
		Server2:       "ntp.tencent.com",
		IntervalHours: 24,
		Enabled:       true,
	}
}

func (s *Server) handleGetNTP(w http.ResponseWriter, r *http.Request) {
	var v string
	if err := s.db.QueryRow("SELECT v FROM settings WHERE k='ntp_config'").Scan(&v); err != nil {
		ok(w, defaultNTP())
		return
	}
	var c NtpConfig
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		ok(w, defaultNTP())
		return
	}
	d := defaultNTP()
	if strings.TrimSpace(c.Server1) == "" {
		c.Server1 = d.Server1
	}
	if strings.TrimSpace(c.Server2) == "" {
		c.Server2 = d.Server2
	}
	if c.IntervalHours <= 0 {
		c.IntervalHours = d.IntervalHours
	}
	ok(w, c)
}

func (s *Server) handlePutNTP(w http.ResponseWriter, r *http.Request) {
	var c NtpConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	c.Server1 = strings.TrimSpace(c.Server1)
	c.Server2 = strings.TrimSpace(c.Server2)
	if c.Server1 == "" {
		fail(w, http.StatusBadRequest, 4000, "NTP 服务器 1 不能为空")
		return
	}
	if c.IntervalHours < 1 || c.IntervalHours > 168 {
		fail(w, http.StatusBadRequest, 4000, "同步间隔须在 1-168 小时之间")
		return
	}
	b, err := json.Marshal(c)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "配置序列化失败")
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings (k, v) VALUES ('ntp_config', ?)
		 ON DUPLICATE KEY UPDATE v = VALUES(v)`, string(b),
	); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "配置保存失败: "+err.Error())
		return
	}
	s.audit(userOf(r), "system_config", "ntp", clientIP(r),
		"更新 NTP 配置：server1="+c.Server1+" interval="+itoa(c.IntervalHours)+"h enabled="+boolToStr(c.Enabled))
	ok(w, c)
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
