package dto

import "time"

// ---- 登录 ----

// LoginReq 邮箱+密码登录（UniSSO password/verify）。
type LoginReq struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录成功：本地会话 JWT + 用户。
type LoginResp struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

type CreateTokenReq struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type CreateTokenResp struct {
	Token     string     `json:"token"`
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type UpdateImageReq struct {
	Visibility string `json:"visibility" binding:"omitempty,oneof=public private"`
}

// UploadJSONReq JSON 方式上传（base64）。
type UploadJSONReq struct {
	Image    string `json:"image" binding:"required"`
	Filename string `json:"filename"`
}

// UploadResp API 上传的返回结构。
type UploadResp struct {
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
	ThumbURL     string `json:"thumb_url"`
	Markdown     string `json:"markdown"`
	HTML         string `json:"html"`
	MimeType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// PageData 通用分页返回。
type PageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// IssueCredentialResp 短期上传凭证签发结果（前端直连 tus 用）。
type IssueCredentialResp struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}
