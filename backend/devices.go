package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ==================== 设备台账（MySQL devices 表） ====================
// GET /api/v1/devices            全部启用设备（所有登录角色可读）
// POST /api/v1/devices           admin 新增
// PUT  /api/v1/devices/{id}      admin 修改
// DELETE /api/v1/devices/{id}    admin 停用（软删 enabled=0）

type Device struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Vendor   string `json:"vendor"`
	Model    string `json:"model"`
	Kind     string `json:"kind"`
	Location string `json:"location"`
	Enabled  bool   `json:"enabled"`
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, name, ip, COALESCE(vendor,''), COALESCE(model,''), COALESCE(kind,''),
		        COALESCE(location,''), enabled
		 FROM devices WHERE enabled=1 ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "设备查询失败")
		return
	}
	defer rows.Close()

	out := make([]Device, 0, 20)
	for rows.Next() {
		var d Device
		var en int
		if rows.Scan(&d.ID, &d.Name, &d.IP, &d.Vendor, &d.Model, &d.Kind, &d.Location, &en) == nil {
			d.Enabled = en == 1
			out = append(out, d)
		}
	}
	ok(w, out)
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	d.IP = strings.TrimSpace(d.IP)
	if d.Name == "" || d.IP == "" {
		fail(w, http.StatusBadRequest, 4000, "设备名称与 IP 不能为空")
		return
	}
	res, err := s.db.Exec(
		`INSERT INTO devices (name, ip, vendor, model, kind, location, enabled) VALUES (?,?,?,?,?,?,1)`,
		d.Name, d.IP, d.Vendor, d.Model, d.Kind, d.Location,
	)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "设备创建失败: "+err.Error())
		return
	}
	id, _ := res.LastInsertId()
	d.ID = id
	d.Enabled = true
	s.audit(userOf(r), "create_device", d.IP, clientIP(r), "新增设备 "+d.Name)
	ok(w, d)
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "设备 ID 无效")
		return
	}
	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		fail(w, http.StatusBadRequest, 4000, "请求体格式错误")
		return
	}
	if _, err := s.db.Exec(
		`UPDATE devices SET name=?, ip=?, vendor=?, model=?, kind=?, location=? WHERE id=?`,
		d.Name, d.IP, d.Vendor, d.Model, d.Kind, d.Location, id,
	); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "设备更新失败: "+err.Error())
		return
	}
	d.ID = id
	d.Enabled = true
	s.audit(userOf(r), "update_device", d.IP, clientIP(r), "修改设备 "+d.Name)
	ok(w, d)
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, 4000, "设备 ID 无效")
		return
	}
	// 审计场景：软删（enabled=0），保留历史引用
	if _, err := s.db.Exec("UPDATE devices SET enabled=0 WHERE id=?", id); err != nil {
		fail(w, http.StatusInternalServerError, 5000, "设备删除失败: "+err.Error())
		return
	}
	s.audit(userOf(r), "delete_device", strconv.FormatInt(id, 10), clientIP(r), "停用设备 #"+strconv.FormatInt(id, 10))
	ok(w, nil)
}

// ==================== 留存策略信息 ====================
// GET /api/v1/retention
// 返回: ttl_days / 存储元信息 / ClickHouse 各日期分区真实占用（行数·磁盘·过期倒计时）
func (s *Server) handleRetention(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// ClickHouse active parts 按日期分区聚合（真实占用）
	type part struct {
		Key      string `json:"key"`
		Date     string `json:"date"`
		Rows     uint64 `json:"rows"`
		Bytes    uint64 `json:"bytes"`
		DaysLeft int64  `json:"days_left"` // 距 TTL 过期天数（负数=已过期待清理）
	}
	parts := make([]part, 0, 40)
	totalBytes := uint64(0)
	now := time.Now()
	if s.ch != nil {
		if rows, err := s.ch.Query(ctx,
			`SELECT partition, sum(rows), sum(bytes_on_disk)
			 FROM system.parts
			 WHERE database='audit' AND table='audit_logs' AND active
			 GROUP BY partition ORDER BY partition`); err == nil {
			for rows.Next() {
				var p part
				var key string
				var b uint64
				if rows.Scan(&key, &p.Rows, &b) == nil {
					p.Key = key
					p.Bytes = b
					totalBytes += b
					if t, err := time.Parse("20060102", key); err == nil {
						p.Date = t.Format("2006-01-02")
						exp := t.AddDate(0, 0, s.cfg.RetentionDays)
						p.DaysLeft = int64(exp.Sub(now).Hours() / 24)
					}
					parts = append(parts, p)
				}
			}
			rows.Close()
		}
	}
	ok(w, map[string]any{
		"ttl_days":        s.cfg.RetentionDays,
		"storage_engine":  "MergeTree",
		"compression":     "ZSTD",
		"partition_field": "toYYYYMMDD(ts)",
		"indexes":         []string{"ngrambf_v1(message)", "bloom_filter(source_ip)"},
		"meta_backend":    "MySQL",
		"total_bytes":     totalBytes,
		"partitions":      parts,
	})
}

// ==================== 用户列表（管理员） ====================
// GET /api/v1/users
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, username, display_name, role, status, COALESCE(last_login_at, ''),
		        (lock_until IS NOT NULL AND lock_until > NOW()) FROM users ORDER BY id`)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "用户查询失败")
		return
	}
	defer rows.Close()

	type userRow struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Display   string `json:"display_name"`
		Role      string `json:"role"`
		Status    int    `json:"status"`
		LastLogin string `json:"last_login_at"`
		Locked    bool   `json:"locked"`
	}
	out := make([]userRow, 0, 20)
	for rows.Next() {
		var ur userRow
		if rows.Scan(&ur.ID, &ur.Username, &ur.Display, &ur.Role, &ur.Status, &ur.LastLogin, &ur.Locked) == nil {
			out = append(out, ur)
		}
	}
	ok(w, out)
}
