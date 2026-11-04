package model

import "time"

const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// Image 一张已上传的图片。Path/ThumbPath 为相对存储根目录的路径，不直接暴露，
// 对外通过 URL/ThumbURL 输出直链。
type Image struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	OriginalName string    `gorm:"size:255" json:"original_name"`
	Path         string    `gorm:"uniqueIndex;size:512;not null" json:"-"`
	ThumbPath    string    `gorm:"size:512" json:"-"`
	TusUploadID  string    `gorm:"size:64;index" json:"-"`
	MimeType     string    `gorm:"size:64" json:"mime_type"`
	Size         int64     `json:"size"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	SHA256       string    `gorm:"size:64;index" json:"-"`
	Visibility   string    `gorm:"size:16;default:public;index" json:"visibility"`
	CreatedAt    time.Time `json:"created_at"`

	// 非持久化字段，由 service 填充。
	URL      string `gorm:"-" json:"url"`
	ThumbURL string `gorm:"-" json:"thumb_url"`
}
