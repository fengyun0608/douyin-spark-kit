// 伙伴 SparkPal 服务端入口。
//
// 单一进制，官方与自建的差异全靠环境变量：
//   MODE=cloud  官方托管（要邀请码、有额度、可计费）
//   MODE=self   自建（无账号、无计费、全放行）
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fengyun0608/sparkpal/server/internal/api"
	"github.com/fengyun0608/sparkpal/server/internal/config"
	"github.com/fengyun0608/sparkpal/server/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	store.SetCookieKey(cfg.Secret)

	st, err := store.Open(cfg.DBDSN)
	if err != nil {
		slog.Error("数据库连接失败", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	h := api.NewRouter(cfg, st)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("伙伴服务已启动",
			"port", cfg.Port, "mode", cfg.Mode,
			"official", cfg.Official, "billing", cfg.Billing,
			"needPairCode", cfg.PairCode != "")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("监听失败", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	slog.Info("已退出")
}
