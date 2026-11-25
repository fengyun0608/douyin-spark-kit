// Package api 组装路由与处理器。
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/fengyun0608/sparkpal/server/internal/config"
	"github.com/fengyun0608/sparkpal/server/internal/douyin"
	"github.com/fengyun0608/sparkpal/server/internal/store"
)

// Server 持有依赖。
type Server struct {
	cfg config.Config
	st  *store.Store
	dy  *douyin.Manager
}

// NewRouter 返回全部路由。
func NewRouter(cfg config.Config, st *store.Store) http.Handler {
	s := &Server{cfg: cfg, st: st, dy: douyin.NewManager(st.DB())}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/spark/health", s.health)
	mux.HandleFunc("GET /api/spark/meta", s.meta)

	mux.HandleFunc("POST /api/spark/auth/invite/check", s.inviteCheck)
	mux.HandleFunc("POST /api/spark/auth/sms/send", s.smsSend)
	mux.HandleFunc("POST /api/spark/auth/sms/verify", s.smsVerify)
	mux.HandleFunc("POST /api/spark/auth/pair", s.pair)
	mux.HandleFunc("GET /api/spark/auth/me", s.me)

	// 抖音绑定
	mux.HandleFunc("POST /api/spark/bind/qr/enqueue", s.bindQR)
	mux.HandleFunc("GET /api/spark/bind/qr/status", s.bindStatus)
	mux.HandleFunc("GET /api/spark/accounts", s.listAccounts)

	// 兜底：未知路径回 JSON，不要吐 nginx 的 HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, http.StatusNotFound, "not_found", "接口不存在： "+r.URL.Path)
	})

	return withCORS(withLog(mux))
}

/* ── 中间件 ── */

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path,
			"ms", time.Since(t0).Milliseconds(), "ip", clientIP(r))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		return strings.Split(v, ",")[0]
	}
	return r.RemoteAddr
}

/* ── 响应helper ── */

func ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": false, "code": code, "message": msg,
	})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
