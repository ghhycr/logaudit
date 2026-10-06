package main

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ==================== 用户管理（系统设置 · 用户管理，仅 admin） ====================
// POST /api/v1/users                     创建用户
// PUT  /api/v1/users/{id}                修改（显示名/角色/状态）
// DELETE /api/v1/users/{id}              停用（软删 status=0，保留审计轨迹）
// POST /api/v1/users/{id}/reset-password 重置密码（不传则随机生成并返回一次）

func validRole(role string) bool {
	return role == "admin" || role == "viewer" || role == "auditor"
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if len(req.Username) < 3 || len(req.Username) > 64 {
		fail(w, http.StatusBadRequest, 4000, "用户名长度须在 3-64 字符之间")
		return
	}
	if len(req.Password) < 8 {
		fail(w, http.StatusBadRequest, 4000, "密码长度不得少于 8 位（等保三级口令策略）")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	if !validRole(req.Role) {
		fail(w, http.StatusBadRequest, 4000, "角色非法（可选 admin / viewer / auditor）")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "密码哈希失败")
		return
	}
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, display_name, role, status) VALUES (?,?,?,?,1)`,
		req.Username, hash, req.DisplayName, req.Role,
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "用户创建失败（用户名可能已存在）")
		return
	}
	id, _ := res.LastInsertId()
	s.audit(userOf(r), "create_user", req.Username, clientIP(r), "创建用户 "+req.Username+"（角色 "+req.Role+"）")
	ok(w, map[string]any{"id": id, "username": req.Username})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "用户 ID 无效")
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      *int   `json:"status"` // 1 启用 / 0 停用
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}

	var targetName, targetRole string
	if err := s.db.QueryRow("SELECT username, role FROM users WHERE id=?", id).Scan(&targetName, &targetRole); err != nil {
		fail(w, http.StatusNotFound, 4040, "用户不存在")
		return
	}
	me := userOf(r)

	// 等保安全保护：不允许修改自己的角色或停用自己的账号
	if me == targetName {
		if req.Role != "" && req.Role != targetRole {
			fail(w, http.StatusForbidden, 4030, "不能修改自己的角色")
			return
		}
		if req.Status != nil && *req.Status != 1 {
			fail(w, http.StatusForbidden, 4030, "不能停用自己的账号")
			return
		}
	}
	// 保护最后一个启用管理员
	if req.Status != nil && *req.Status == 0 && targetRole == "admin" {
		var n int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin' AND status=1").Scan(&n)
		if n <= 1 {
			fail(w, http.StatusForbidden, 4030, "不能停用最后一个管理员账号")
			return
		}
	}
	if req.Role != "" && !validRole(req.Role) {
		fail(w, http.StatusBadRequest, 4000, "角色非法（可选 admin / viewer / auditor）")
		return
	}

	newRole := req.Role
	if newRole == "" {
		newRole = targetRole
	}
	// status 指针为 nil 时不修改状态
	if req.Status == nil {
		if _, err := s.db.Exec(
			"UPDATE users SET display_name=?, role=? WHERE id=?",
			req.DisplayName, newRole, id,
		); err != nil {
			fail(w, http.StatusInternalServerError, 5000, "用户更新失败: "+err.Error())
			return
		}
	} else {
		if _, err := s.db.Exec(
			"UPDATE users SET display_name=?, role=?, status=? WHERE id=?",
			req.DisplayName, newRole, *req.Status, id,
		); err != nil {
			fail(w, http.StatusInternalServerError, 5000, "用户更新失败: "+err.Error())
			return
		}
	}
	s.audit(me, "update_user", targetName, clientIP(r),
		"更新用户 "+targetName+"（角色 "+targetRole+"→"+newRole+"）")
	ok(w, map[string]any{"id": id, "username": targetName})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "用户 ID 无效")
		return
	}
	var targetName, targetRole string
	if err := s.db.QueryRow("SELECT username, role FROM users WHERE id=?", id).Scan(&targetName, &targetRole); err != nil {
		fail(w, http.StatusNotFound, 4040, "用户不存在")
		return
	}
	me := userOf(r)
	if me == targetName {
		fail(w, http.StatusForbidden, 4030, "不能停用自己的账号")
		return
	}
	if targetRole == "admin" {
		var n int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin' AND status=1").Scan(&n)
		if n <= 1 {
			fail(w, http.StatusForbidden, 4030, "不能停用最后一个管理员账号")
			return
		}
	}
	// 软删：status=0，保留历史引用与审计
	if _, err := s.db.Exec("UPDATE users SET status=0 WHERE id=?", id); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "用户停用失败: "+err.Error())
		return
	}
	s.audit(me, "delete_user", targetName, clientIP(r), "停用用户 "+targetName)
	ok(w, nil)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "用户 ID 无效")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var targetName string
	if err := s.db.QueryRow("SELECT username FROM users WHERE id=?", id).Scan(&targetName); err != nil {
		fail(w, http.StatusNotFound, 4040, "用户不存在")
		return
	}
	pw := strings.TrimSpace(req.Password)
	if pw == "" {
		pw = randomPassword(14)
	}
	if len(pw) < 8 {
		fail(w, http.StatusBadRequest, 4000, "密码长度不得少于 8 位")
		return
	}
	hash, err := hashPassword(pw)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "密码哈希失败")
		return
	}
	if _, err := s.db.Exec(
		"UPDATE users SET password_hash=?, fail_count=0, lock_until=NULL WHERE id=?",
		hash, id,
	); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "密码重置失败: "+err.Error())
		return
	}
	s.audit(userOf(r), "reset_password", targetName, clientIP(r), "重置用户 "+targetName+" 的密码")
	ok(w, map[string]any{"username": targetName, "password": pw})
}

// randomPassword 生成强口令（大小写+数字+符号，等保三级口令复杂度）
func randomPassword(n int) string {
	const upper = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const lower = "abcdefghijkmnpqrstuvwxyz"
	const digit = "23456789"
	const symbol = "!@#$%^&*"
	groups := []string{upper, lower, digit, symbol}
	buf := make([]byte, 0, n)
	for _, g := range groups {
		b := make([]byte, 1)
		_, _ = rand.Read(b)
		buf = append(buf, g[int(b[0])%len(g)])
	}
	for len(buf) < n {
		b := make([]byte, 1)
		_, _ = rand.Read(b)
		all := upper + lower + digit + symbol
		buf = append(buf, all[int(b[0])%len(all)])
	}
	return string(buf[:n])
}
