package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imagebed/blob"
	"imagebed/config"
	"imagebed/core"
	"imagebed/util"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if err := ensureJWTSecret(cfg); err != nil {
		log.Fatalf("初始化 JWT 密钥失败: %v", err)
	}

	db, err := core.InitDB(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	store, err := blob.NewFromEnv(cfg.DataDir())
	if err != nil {
		log.Fatalf("初始化对象存储失败: %v", err)
	}
	log.Printf("二进制存储后端: %s（SQLite/JWT 仍在 DATA_DIR=%s）", store.Backend(), cfg.DataDir())

	r := core.SetupRouter(cfg, db, store)

	addr := "0.0.0.0:" + cfg.Server.Port
	// 生产级超时：ReadHeaderTimeout 防慢速建连（slowloris）；ReadTimeout 需覆盖 2MB 分片
	// 在慢网下的读取（5min ≈ 7KB/s 极低速）；WriteTimeout / IdleTimeout 兜底长连接。
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("图床服务已启动: http://%s （须 0.0.0.0，供 sseinfra frp / 网关访问）", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// ensureJWTSecret 保证 JWT 密钥非空且稳定：
// 显式配置（环境变量 / config.yaml 非占位值）时沿用；否则生成随机密钥并持久化到数据目录，
// 这样容器重建、服务重启后登录态仍然有效。
func ensureJWTSecret(cfg *config.Config) error {
	const placeholder = "change-me-to-a-long-random-secret-string"
	if cfg.JWT.Secret != "" && cfg.JWT.Secret != placeholder {
		return nil
	}

	if err := os.MkdirAll(cfg.DataDir(), 0o755); err != nil {
		return err
	}
	secretPath := filepath.Join(cfg.DataDir(), ".jwt_secret")
	if b, err := os.ReadFile(secretPath); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			cfg.JWT.Secret = s
			return nil
		}
	}

	secret, err := util.RandomHex(32)
	if err != nil {
		return err
	}
	if err := os.WriteFile(secretPath, []byte(secret), 0o600); err != nil {
		return err
	}
	cfg.JWT.Secret = secret
	log.Printf("已生成并持久化随机 JWT 密钥: %s", secretPath)
	return nil
}
