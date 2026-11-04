package model

import "time"

// Token 个人 API Token。只保存 Token 原文的 SHA-256，原文仅在创建时返回一次。
type Token struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	Name       string     `gorm:"size:128" json:"name"`
	TokenHash  string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
