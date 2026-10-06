package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

// ==================== 基础设置（系统设置 · 基础设置，仅 admin） ====================
// GET /api/v1/settings/base   读取配置（settings 表 base_config，缺省给默认值，Redis 缓存 60s）
// PUT /api/v1/settings/base   保存配置（保存后立即清缓存，登录/签发/改密策略即时生效）
//
// 生效点：
//   - 登录失败处理：handleLogin 锁定阈值/锁定时长
//   - 会话超时：登录/刷新签发 access token 的 TTL（前端空闲登出同步使用登录响应值）
//   - 密码复杂度：创建用户 / 重置密码 / 修改密码校验
//   - 密码有效期：登录成功后返回 need_change_password（前端提示改密）
//   - 系统访问白名单：登录时来源 IP 校验（空 = 不限制）

type BaseSettings struct {
	// 登录失败处理（等保三级：连续失败锁定）
	MaxFailCount  int `json:"max_fail_count"`  // 1-10，默认 5
	LockMinutes   int `json:"lock_minutes"`    // 1-120，默认 10
	// 会话超时（分钟，空闲自动登出）
	SessionTimeoutMinutes int `json:"session_timeout_minutes"` // 5-240，默认 15
	// 密码复杂度
	PwdMinLength      int  `json:"pwd_min_length"`        // 8-32，默认 8
	PwdRequireUpper   bool `json:"pwd_require_upper"`     // 默认 true
	PwdRequireLower   bool `json:"pwd_require_lower"`     // 默认 true
	PwdRequireDigit   bool `json:"pwd_require_digit"`     // 默认 true
	PwdRequireSpecial bool `json:"pwd_require_special"`   // 默认 true
	// 密码有效期（天，0 = 不限期）
	PwdExpireDays int `json:"pwd_expire_days"` // 30-365，默认 90
	// 系统访问白名单（逗号分隔 IP / CIDR，空 = 不限制）
	WhitelistIps string `json:"whitelist_ips"`
}

func defaultBase() BaseSettings {
	return BaseSettings{
		MaxFailCount:          5,
		LockMinutes:           10,
		SessionTimeoutMinutes: 15,
		PwdMinLength:          8,
		PwdRequireUpper:       true,
		PwdRequireLower:       true,
		PwdRequireDigit:       true,
		PwdRequireSpecial:     true,
		PwdExpireDays:         90,
		WhitelistIps:          "",
	}
}

// loadBaseSettings 读取基础设置（Redis 缓存 60s；失败降级直接查库）
func (s *Server) loadBaseSettings() BaseSettings {
	d := defaultBase()
	if s.rdb != nil {
		if v, err := s.rdb.Get(context.Background(), "settings:base").Result(); err == nil && v != "" {
			var c BaseSettings
			if json.Unmarshal([]byte(v), &c) == nil {
				return normalizeBase(c, d)
			}
		}
	}
	var v string
	if err := s.db.QueryRow("SELECT v FROM settings WHERE k='base_config'").Scan(&v); err != nil {
		return d
	}
	var c BaseSettings
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return d
	}
	c = normalizeBase(c, d)
	if s.rdb != nil {
		if b, err := json.Marshal(c); err == nil {
			s.rdb.Set(context.Background(), "settings:base", b, 60*time.Second)
		}
	}
	return c
}

func normalizeBase(c, d BaseSettings) BaseSettings {
	if c.MaxFailCount < 1 || c.MaxFailCount > 10 {
		c.MaxFailCount = d.MaxFailCount
	}
	if c.LockMinutes < 1 || c.LockMinutes > 120 {
		c.LockMinutes = d.LockMinutes
	}
	if c.SessionTimeoutMinutes < 5 || c.SessionTimeoutMinutes > 240 {
		c.SessionTimeoutMinutes = d.SessionTimeoutMinutes
	}
	if c.PwdMinLength < 8 || c.PwdMinLength > 32 {
		c.PwdMinLength = d.PwdMinLength
	}
	if c.PwdExpireDays < 0 || c.PwdExpireDays > 365 {
		c.PwdExpireDays = d.PwdExpireDays
	}
	return c
}

// validatePasswordWithPolicy 按基础设置中的密码复杂度策略校验
func (s *Server) validatePasswordWithPolicy(pw string) error {
	bs := s.loadBaseSettings()
	if len(pw) < bs.PwdMinLength {
		return &badPwd{msg: "密码长度不得少于 " + itoa(bs.PwdMinLength) + " 位"}
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
	var need []string
	if bs.PwdRequireUpper && !hasUpper {
		need = append(need, "大写字母")
	}
	if bs.PwdRequireLower && !hasLower {
		need = append(need, "小写字母")
	}
	if bs.PwdRequireDigit && !hasDigit {
		need = append(need, "数字")
	}
	if bs.PwdRequireSpecial && !hasSpecial {
		need = append(need, "特殊字符")
	}
	if len(need) > 0 {
		return &badPwd{msg: "密码须包含：" + strings.Join(need, "、")}
	}
	return nil
}

type badPwd struct{ msg string }

func (e *badPwd) Error() string { return e.msg }

// ipAllowed 判断来源 IP 是否在白名单（支持 IPv4/IPv6 精确 IP 与 CIDR；空名单 = 全部允许）
func ipAllowed(ip string, whitelist string) bool {
	if strings.TrimSpace(whitelist) == "" {
		return true
	}
	ip = strings.TrimSpace(ip)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, item := range strings.Split(whitelist, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(item); err == nil {
			if cidr.Contains(parsed) {
				return true
			}
			continue
		}
		if p := net.ParseIP(item); p != nil && p.Equal(parsed) {
			return true
		}
	}
	return false
}

func (s *Server) handleGetBaseSettings(w http.ResponseWriter, r *http.Request) {
	ok(w, s.loadBaseSettings())
}

func (s *Server) handlePutBaseSettings(w http.ResponseWriter, r *http.Request) {
	var c BaseSettings
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	d := defaultBase()
	if c.MaxFailCount < 1 || c.MaxFailCount > 10 {
		fail(w, http.StatusBadRequest, 4000, "连续失败次数须在 1-10 之间")
		return
	}
	if c.LockMinutes < 1 || c.LockMinutes > 120 {
		fail(w, http.StatusBadRequest, 4000, "锁定时长须在 1-120 分钟之间")
		return
	}
	if c.SessionTimeoutMinutes < 5 || c.SessionTimeoutMinutes > 240 {
		fail(w, http.StatusBadRequest, 4000, "会话超时须在 5-240 分钟之间")
		return
	}
	if c.PwdMinLength < 8 || c.PwdMinLength > 32 {
		fail(w, http.StatusBadRequest, 4000, "密码最小长度须在 8-32 之间")
		return
	}
	if c.PwdExpireDays < 0 || c.PwdExpireDays > 365 {
		fail(w, http.StatusBadRequest, 4000, "密码有效期须在 0-365 天之间（0 为不限期）")
		return
	}
	if !c.PwdRequireUpper && !c.PwdRequireLower && !c.PwdRequireDigit && !c.PwdRequireSpecial {
		fail(w, http.StatusBadRequest, 4000, "密码复杂度至少须启用一类字符要求")
		return
	}
	c = normalizeBase(c, d)
	b, err := json.Marshal(c)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "配置序列化失败")
		return
	}
	if _, err := s.db.Exec(
		`INSERT INTO settings (k, v) VALUES ('base_config', ?)
		 ON DUPLICATE KEY UPDATE v = VALUES(v)`, string(b),
	); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "配置保存失败: "+err.Error())
		return
	}
	if s.rdb != nil {
		s.rdb.Del(context.Background(), "settings:base")
	}
	s.audit(userOf(r), "system_config", "base", clientIP(r),
		"更新基础设置：失败阈值="+itoa(c.MaxFailCount)+" 锁定时长="+itoa(c.LockMinutes)+
			"m 会话超时="+itoa(c.SessionTimeoutMinutes)+"min 密码有效期="+itoa(c.PwdExpireDays)+"d")
	ok(w, c)
}
