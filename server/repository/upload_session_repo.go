package repository

import (
	"time"

	"imagebed/model"

	"gorm.io/gorm"
)

type UploadSessionRepo struct {
	db *gorm.DB
}

func NewUploadSessionRepo(db *gorm.DB) *UploadSessionRepo {
	return &UploadSessionRepo{db: db}
}

// CountActive 返回指定用户在活跃窗口内（updated_at >= activeSince）的上传会话数。
// 活跃窗口外的僵尸会话（创建后长时间无 PATCH）不再计入并发额度。
func (r *UploadSessionRepo) CountActive(userID uint, activeSince time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&model.UploadSession{}).
		Where("user_id = ? AND updated_at >= ?", userID, activeSince).Count(&n).Error
	return n, err
}

// CountAll 返回全平台活跃窗口内（updated_at >= activeSince）的上传会话总数（全局兜底闸用）。
func (r *UploadSessionRepo) CountAll(activeSince time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&model.UploadSession{}).
		Where("updated_at >= ?", activeSince).Count(&n).Error
	return n, err
}

// ActiveByUser 返回每个用户的活跃上传会话数（观测日志用）。
func (r *UploadSessionRepo) ActiveByUser() (map[uint]int64, error) {
	type row struct {
		UserID uint  `gorm:"column:user_id"`
		N      int64 `gorm:"column:n"`
	}
	var rows []row
	err := r.db.Model(&model.UploadSession{}).
		Select("user_id, COUNT(*) AS n").
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int64, len(rows))
	for _, x := range rows {
		m[x.UserID] = x.N
	}
	return m, nil
}

func (r *UploadSessionRepo) Create(tusID string, userID uint) error {
	return r.db.Create(&model.UploadSession{TusID: tusID, UserID: userID}).Error
}

func (r *UploadSessionRepo) Delete(tusID string) error {
	return r.db.Where("tus_id = ?", tusID).Delete(&model.UploadSession{}).Error
}

// Touch 刷新会话活跃时间（PATCH 分片时调用）。
// 仅当距上次活跃超过 30s 才写库，降低 SQLite 写放大（活跃窗口 10min，30s 粒度足够）。
func (r *UploadSessionRepo) Touch(tusID string) error {
	return r.db.Model(&model.UploadSession{}).
		Where("tus_id = ? AND updated_at < ?", tusID, time.Now().Add(-30*time.Second)).
		Update("updated_at", time.Now()).Error
}

func (r *UploadSessionRepo) DeleteExpired(before time.Time) error {
	return r.db.Where("updated_at < ?", before).Delete(&model.UploadSession{}).Error
}
