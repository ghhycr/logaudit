package main

import (
	"context"
	"log"
	"net/http"
	"strings"
)

type ctxKey int

const (
	ctxUID    ctxKey = iota
	ctxUser
	ctxRole
	ctxCSRF
	ctxClaims
)

// requireAuth Bearer JWT 鉴权中间件（等保三级：令牌校验）
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			fail(w, http.StatusUnauthorized, 4010, "未提供访问令牌")
			return
		}
		claims, err := parseAccess(s.cfg, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			fail(w, http.StatusUnauthorized, 4010, "令牌无效或已过期")
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxUID, claims.UserID)
		ctx = context.WithValue(ctx, ctxUser, claims.Username)
		ctx = context.WithValue(ctx, ctxRole, claims.Role)
		ctx = context.WithValue(ctx, ctxCSRF, r.Header.Get("X-XSRF-TOKEN"))
		ctx = context.WithValue(ctx, ctxClaims, claims)
		next(w, r.WithContext(ctx))
	}
}

// requireRole 角色权限中间件（等保三级：最小权限）
// 用法: s.requireAuth(s.requireRole("admin", handler))
func (s *Server) requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cur := r.Context().Value(ctxRole).(string)
		if cur != role && cur != "admin" {
			fail(w, http.StatusForbidden, 4030, "无权限执行该操作（需要 "+role+" 角色）")
			return
		}
		next(w, r)
	}
}

// recoverPanic 防止异常导致整个服务退出
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				fail(w, http.StatusInternalServerError, 5000, "系统繁忙，请稍后再试")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders 服务端补充安全响应头（与 nginx 叠加）
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
