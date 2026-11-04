package core

import (
	"os"
	"path/filepath"

	"imagebed/config"
	"imagebed/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 初始化 SQLite（自动建库建表）并确保存储目录存在。
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	for _, dir := range []string{
		filepath.Dir(cfg.Database.Path),
		cfg.ImagesDir(),
		cfg.ThumbsDir(),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Image{}, &model.UploadSession{}); err != nil {
		return nil, err
	}

	// 兼容早期版本：最早的 users 表含 username / password_hash（本地账号体系），
	// 现已改为纯 SSE Market OAuth 登录（用户只有 oauth_id，无 username）。
	// GORM AutoMigrate 不会删除旧列，而 username 带 UNIQUE 约束又导致 SQLite 无法直接
	// DROP COLUMN，故检测到旧列时直接重建 users 表。旧本地账号本就无 oauth_id、无法再登录，
	// 可安全丢弃；图片 / Token 表不受影响。
	m := db.Migrator()
	if m.HasColumn(&model.User{}, "username") || m.HasColumn(&model.User{}, "password_hash") {
		if err := m.DropTable(&model.User{}); err != nil {
			return nil, err
		}
		if err := db.AutoMigrate(&model.User{}); err != nil {
			return nil, err
		}
	}

	return db, nil
}
