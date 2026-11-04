package model

import "time"

// UploadSession 断点续传进行中的任务，用于限制每用户并发上传数并跨重启保留状态。
type UploadSession struct {
	TusID     string    `gorm:"column:tus_id;primaryKey;size:64" json:"tus_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UploadSession) TableName() string { return "upload_sessions" }
