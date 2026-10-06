package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// ==================== 响应包装 ====================

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Message: message, Data: data})
}

func ok(w http.ResponseWriter, data any) { writeJSON(w, http.StatusOK, 0, "ok", data) }

func fail(w http.ResponseWriter, status, code int, message string) {
	writeJSON(w, status, code, message, nil)
}

// clientIP 提取真实客户端 IP（nginx 反代后优先 X-Forwarded-For）
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	return r.RemoteAddr
}

// ==================== 登录 ====================

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ---- Redis 登录防爆破辅助（等保三级：连续失败锁定）----

// redisLoginBlocked Redis 快速锁定检查（存在 lock 键即锁定）
func (s *Server) redisLoginBlocked(username string) bool {
	if s.rdb == nil {
		return false
	}
	n, err := s.rdb.Exists(context.Background(), "lock:"+username).Result()
	return err == nil && n > 0
}

// redisFailIncr Redis 失败计数：达到阈值则写入锁定键并返回 true
func (s *Server) redisFailIncr(username string) (bool, error) {
	if s.rdb == nil {
		return false, nil
	}
	ctx := context.Background()
	key := "fail:" + username
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, 10*time.Minute) // 计数窗口 10 分钟
	}
	if n >= int64(s.cfg.LockThreshold) {
		s.rdb.Del(ctx, key)
		lockDur := time.Duration(s.cfg.LockMinutes) * time.Minute
		return true, s.rdb.Set(ctx, "lock:"+username, "1", lockDur).Err()
	}
	return false, nil
}

// redisLoginClear 登录成功清除失败计数与锁定
func (s *Server) redisLoginClear(username string) {
	if s.rdb == nil {
		return
	}
	ctx := context.Background()
	s.rdb.Del(ctx, "fail:"+username, "lock:"+username)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		fail(w, http.StatusBadRequest, 4000, "用户名与密码不能为空")
		return
	}

	// 系统访问白名单（基础设置；空名单 = 不限制）
	bs := s.loadBaseSettings()
	srcIP := clientIP(r)
	if !ipAllowed(srcIP, bs.WhitelistIps) {
		s.audit(req.Username, "login", "", srcIP, "登录被拒：来源 IP 不在系统访问白名单内")
		fail(w, http.StatusForbidden, 4033, "当前访问来源不在系统白名单内，请联系管理员")
		return
	}

	// Redis 快速锁定拦截（未接入 Redis 时走 MySQL 检查）
	if s.redisLoginBlocked(req.Username) {
		fail(w, http.StatusLocked, 4230, "连续登录失败次数过多，账号已锁定（等保三级安全策略）")
		return
	}

	u, err := getUserByName(s.db, req.Username)
	if err != nil {
		log.Printf("查询用户失败: %v", err)
		fail(w, http.StatusInternalServerError, 5000, "系统繁忙，请稍后再试")
		return
	}
	if u == nil {
		s.audit(req.Username, "login", "", srcIP, "登录失败：用户不存在")
		fail(w, http.StatusUnauthorized, 4010, "用户名或密码错误")
		return
	}
	if u.Status != 1 {
		fail(w, http.StatusForbidden, 4030, "账号已被停用")
		return
	}
	// 锁定检查（等保三级：连续失败锁定，阈值/时长来自基础设置）
	now := time.Now()
	if u.LockUntil != nil && u.LockUntil.After(now) {
		remain := int(u.LockUntil.Sub(now).Minutes()) + 1
		fail(w, http.StatusLocked, 4230, "连续登录失败次数过多，账号已锁定，请 "+itoa(remain)+" 分钟后再试")
		return
	}

	if !checkPassword(u.PasswordHash, req.Password) {
		// Redis 计数（防爆破高频写入 MySQL 的压力优化层）
		redisLocked, _ := s.redisFailIncr(req.Username)

		newCount := u.FailCount + 1
		var lockUntil *time.Time
		if newCount >= bs.MaxFailCount {
			t := now.Add(time.Duration(bs.LockMinutes) * time.Minute)
			lockUntil = &t
			newCount = 0
		}
		if _, err := s.db.Exec(
			"UPDATE users SET fail_count=?, lock_until=? WHERE id=?",
			newCount, lockUntil, u.ID,
		); err != nil {
			log.Printf("更新失败计数失败: %v", err)
		}
		if lockUntil != nil || redisLocked {
			s.audit(u.Username, "login", "", srcIP, "登录失败：连续"+itoa(bs.MaxFailCount)+"次错误触发锁定")
			fail(w, http.StatusLocked, 4230, "连续登录失败次数过多，账号已锁定 "+itoa(bs.LockMinutes)+" 分钟（等保三级安全策略）")
			return
		}
		s.audit(u.Username, "login", "", srcIP, "登录失败：密码错误")
		fail(w, http.StatusUnauthorized, 4010, "用户名或密码错误（剩余尝试次数："+itoa(bs.MaxFailCount-newCount)+"）")
		return
	}

	// 登录成功
	s.redisLoginClear(req.Username)
	if _, err := s.db.Exec(
		"UPDATE users SET fail_count=0, lock_until=NULL, last_login_at=? WHERE id=?",
		now, u.ID,
	); err != nil {
		log.Printf("更新登录状态失败: %v", err)
	}
	u.FailCount = 0
	u.LockUntil = nil
	u.LastLoginAt = &now

	// 密码有效期检查（基础设置；到期返回 need_change_password 提示）
	needPwdChange := false
	if bs.PwdExpireDays > 0 && u.PasswordChangedAt != nil {
		if time.Since(*u.PasswordChangedAt) > time.Duration(bs.PwdExpireDays)*24*time.Hour {
			needPwdChange = true
		}
	}

	// 签发令牌（会话超时 = 基础设置 session_timeout_minutes）
	csrf, err := randomHex(16)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "系统繁忙，请稍后再试")
		return
	}
	sessTTL := time.Duration(bs.SessionTimeoutMinutes) * time.Minute
	access, err := signAccessWithTTL(s.cfg, u, csrf, sessTTL)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "令牌签发失败")
		return
	}
	refresh, err := s.issueRefresh(u.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "刷新令牌签发失败")
		return
	}

	s.audit(u.Username, "login", "", srcIP, "登录成功")
	ok(w, map[string]any{
		"access_token":          access,
		"refresh_token":         refresh,
		"expires_in":            int(sessTTL.Seconds()),
		"csrf_token":            csrf,
		"session_timeout_minutes": bs.SessionTimeoutMinutes,
		"need_change_password":  needPwdChange,
		"user":                  u.public(),
	})
}

// issueRefresh 生成 refresh token（32 字节随机），仅存 SHA-256 哈希
func (s *Server) issueRefresh(userID int64) (string, error) {
	tok, err := randomHex(32)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(tok))
	if _, err := s.db.Exec(
		"INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?,?,?)",
		userID, hex.EncodeToString(hash[:]), time.Now().Add(s.cfg.RefreshTTL),
	); err != nil {
		return "", err
	}
	return tok, nil
}

// ==================== 刷新 ====================

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		fail(w, http.StatusBadRequest, 4000, "refresh_token 缺失")
		return
	}
	hash := sha256.Sum256([]byte(req.RefreshToken))
	h := hex.EncodeToString(hash[:])

	var userID int64
	var expiresAt time.Time
	var revoked int
	err := s.db.QueryRow(
		"SELECT user_id, expires_at, revoked FROM refresh_tokens WHERE token_hash=?", h,
	).Scan(&userID, &expiresAt, &revoked)
	if err != nil || revoked == 1 || time.Now().After(expiresAt) {
		fail(w, http.StatusUnauthorized, 4011, "刷新令牌无效或已过期")
		return
	}

	u, err := getUserByID(s.db, userID)
	if err != nil || u == nil || u.Status != 1 {
		fail(w, http.StatusUnauthorized, 4011, "账号状态异常")
		return
	}
	csrf, err := randomHex(16)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "系统繁忙")
		return
	}
	access, err := signAccess(s.cfg, u, csrf)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "令牌签发失败")
		return
	}
	ok(w, map[string]any{"access_token": access, "csrf_token": csrf, "expires_in": int(s.cfg.AccessTTL.Seconds())})
}

// ==================== 登出 ====================

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		hash := sha256.Sum256([]byte(req.RefreshToken))
		_, _ = s.db.Exec("UPDATE refresh_tokens SET revoked=1 WHERE token_hash=?", hex.EncodeToString(hash[:]))
	}
	username := r.Context().Value(ctxUser).(string)
	s.audit(username, "logout", "", r.RemoteAddr, "退出登录")
	ok(w, nil)
}

// ==================== 当前用户 ====================

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	uid := r.Context().Value(ctxUID).(int64)
	u, err := getUserByID(s.db, uid)
	if err != nil || u == nil {
		fail(w, http.StatusUnauthorized, 4011, "会话失效，请重新登录")
		return
	}
	ok(w, u.public())
}

// ==================== 修改密码 ====================

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	uid := r.Context().Value(ctxUID).(int64)
	csrf := r.Context().Value(ctxCSRF).(string)
	if !validCSRF(r.Context().Value(ctxClaims).(*Claims), csrf) {
		fail(w, http.StatusForbidden, 4031, "CSRF 校验失败")
		return
	}
	var req changePwdReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	u, err := getUserByID(s.db, uid)
	if err != nil || u == nil {
		fail(w, http.StatusUnauthorized, 4011, "会话失效")
		return
	}
	if !checkPassword(u.PasswordHash, req.OldPassword) {
		s.audit(u.Username, "change_password", "", r.RemoteAddr, "原密码校验失败")
		fail(w, http.StatusForbidden, 4032, "原密码错误")
		return
	}
	if err := s.validatePasswordWithPolicy(req.NewPassword); err != nil {
		fail(w, http.StatusBadRequest, 4001, err.Error())
		return
	}
	hash, err := hashPassword(req.NewPassword)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "系统繁忙")
		return
	}
	if _, err := s.db.Exec("UPDATE users SET password_hash=?, password_changed_at=? WHERE id=?", hash, time.Now(), u.ID); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "密码更新失败")
		return
	}
	// 吊销该用户全部刷新令牌（等保：密码变更后会话失效）
	_, _ = s.db.Exec("UPDATE refresh_tokens SET revoked=1 WHERE user_id=?", u.ID)
	s.audit(u.Username, "change_password", "", r.RemoteAddr, "密码修改成功")
	ok(w, nil)
}

// ==================== 操作审计 ====================

func (s *Server) audit(username, action, target, ip, detail string) {
	_, err := s.db.Exec(
		"INSERT INTO op_audits (username, action, target, detail, ip) VALUES (?,?,?,?,?)",
		username, action, target, detail, ip,
	)
	if err != nil {
		log.Printf("写操作审计失败: %v", err)
	}
}

var _ = errors.New
