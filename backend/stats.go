package main

import (
	"context"
	"net/http"
	"time"
)

// ==================== 统计概览 ====================
// GET /api/v1/stats/overview
// 返回: today_total / last_7d / category_dist / top_sources / top_events

func (s *Server) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var todayTotal uint64
	_ = s.ch.QueryRow(ctx,
		"SELECT count() FROM audit.audit_logs WHERE toDate(ts) = today()",
	).Scan(&todayTotal)

	// 近 7 天趋势
	type dayCount struct {
		Day   string `json:"day"`
		Count uint64 `json:"count"`
	}
	last7 := make([]dayCount, 0, 7)
	rows, err := s.ch.Query(ctx,
		`SELECT toDate(ts) d, count() c FROM audit.audit_logs
		 WHERE ts >= today() - INTERVAL 6 DAY GROUP BY d ORDER BY d`)
	if err == nil {
		for rows.Next() {
			var dc dayCount
			var d time.Time
			if rows.Scan(&d, &dc.Count) == nil {
				dc.Day = d.Format("2006-01-02")
				last7 = append(last7, dc)
			}
		}
		rows.Close()
	}

	// 事件类别分布（按 event_type 聚合，近 7 天）
	type catDist struct {
		Category string `json:"category"`
		Count    uint64 `json:"count"`
	}
	cats := make([]catDist, 0, 10)
	rows, err = s.ch.Query(ctx,
		`SELECT event_type, count() c FROM audit.audit_logs
		 WHERE ts >= today() - INTERVAL 6 DAY GROUP BY event_type ORDER BY c DESC LIMIT 10`)
	if err == nil {
		for rows.Next() {
			var cd catDist
			var et string
			if rows.Scan(&et, &cd.Count) == nil {
				cd.Category = eventCategory(et)
				cats = append(cats, cd)
			}
		}
		rows.Close()
	}

	// TOP 源 IP
	type topSrc struct {
		SourceIP string `json:"source_ip"`
		Count    uint64 `json:"count"`
	}
	srcs := make([]topSrc, 0, 10)
	rows, err = s.ch.Query(ctx,
		`SELECT source_ip, count() c FROM audit.audit_logs
		 WHERE ts >= today() - INTERVAL 6 DAY GROUP BY source_ip ORDER BY c DESC LIMIT 10`)
	if err == nil {
		for rows.Next() {
			var ts topSrc
			if rows.Scan(&ts.SourceIP, &ts.Count) == nil {
				srcs = append(srcs, ts)
			}
		}
		rows.Close()
	}

	// TOP 事件
	type topEv struct {
		EventType string `json:"event_type"`
		Count     uint64 `json:"count"`
	}
	evs := make([]topEv, 0, 10)
	rows, err = s.ch.Query(ctx,
		`SELECT event_type, count() c FROM audit.audit_logs
		 WHERE ts >= today() - INTERVAL 6 DAY GROUP BY event_type ORDER BY c DESC LIMIT 10`)
	if err == nil {
		for rows.Next() {
			var te topEv
			if rows.Scan(&te.EventType, &te.Count) == nil {
				evs = append(evs, te)
			}
		}
		rows.Close()
	}

	ok(w, map[string]any{
		"today_total":  todayTotal,
		"last_7d":      last7,
		"category_dist": cats,
		"top_sources":  srcs,
		"top_events":   evs,
	})
}

// ==================== 登录失败统计 ====================
// GET /api/v1/stats/login-fail（近 7 天，来自 MySQL 操作审计）
func (s *Server) handleLoginFailStats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT DATE(ts) d, COUNT(*) c FROM op_audits
		 WHERE action='login' AND detail LIKE '%失败%' AND ts >= NOW() - INTERVAL 6 DAY
		 GROUP BY d ORDER BY d`)
	if err != nil {
		fail(w, http.StatusInternalServerError, 5000, "统计查询失败")
		return
	}
	defer rows.Close()

	type dayCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	out := make([]dayCount, 0, 7)
	for rows.Next() {
		var dc dayCount
		if rows.Scan(&dc.Day, &dc.Count) == nil {
			out = append(out, dc)
		}
	}
	ok(w, out)
}
