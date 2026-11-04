package model

import "time"

// User 本地用户。身份由 UniSSO 邮箱+密码登录提供，OAuthID 为 OIDC sub，
// 首次登录时 upsert 落库；图片 / Token 均按本地自增 ID 归属。
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OAuthID   string    `gorm:"column:oauth_id;uniqueIndex;size:64;not null" json:"oauth_id"`
	Name      string    `gorm:"size:128" json:"name"`
	Email     string    `gorm:"size:255" json:"email"`
	AvatarURL string    `gorm:"size:512" json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
