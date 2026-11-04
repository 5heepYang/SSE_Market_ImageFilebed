package controller

import (
	"encoding/base64"
	htmlpkg "html"
	"io"
	"log"
	"strings"

	"imagebed/dto"
	"imagebed/middleware"
	"imagebed/response"
	"imagebed/service"
	"imagebed/util"

	"github.com/gin-gonic/gin"
)

type UploadController struct {
	upload *service.UploadService
}

func NewUploadController(upload *service.UploadService) *UploadController {
	return &UploadController{upload: upload}
}

type uploadItem struct {
	name string
	data []byte
}

// Upload API 上传，支持三种输入方式：
//  1. multipart/form-data，字段 file（可多文件）
//  2. 原始二进制 body（Content-Type: image/*）
//  3. JSON：{"image":"<base64>","filename":"x.png"}
func (c *UploadController) Upload(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	ct := ctx.ContentType()

	items, ok := c.parseItems(ctx, ct)
	if !ok {
		return
	}

	results := make([]dto.UploadResp, 0, len(items))
	failed := 0
	for _, it := range items {
		img, err := c.upload.Save(uid, it.name, it.data)
		if err != nil {
			failed++
			log.Printf("API 上传失败 user=%d name=%q: %v", uid, it.name, err)
			continue
		}
		isImage := util.IsImageMime(img.MimeType)
		results = append(results, dto.UploadResp{
			URL:          img.URL,
			OriginalName: img.OriginalName,
			ThumbURL:     img.ThumbURL,
			Markdown:     markdown(img.URL, img.OriginalName, isImage),
			HTML:         html(img.URL, img.OriginalName, isImage),
			MimeType:     img.MimeType,
			Size:         img.Size,
			Width:        img.Width,
			Height:       img.Height,
		})
	}

	if len(results) == 0 {
		response.BadRequest(ctx, "上传失败，请检查文件类型与大小")
		return
	}
	if failed > 0 {
		response.OKWithMessage(ctx, results, "部分文件上传失败")
		return
	}
	response.OK(ctx, results)
}

func (c *UploadController) parseItems(ctx *gin.Context, ct string) ([]uploadItem, bool) {
	switch {
	case strings.HasPrefix(ct, "multipart/form-data"):
		form, err := ctx.MultipartForm()
		if err != nil {
			response.BadRequest(ctx, "解析上传表单失败")
			return nil, false
		}
		files := form.File["file"]
		if len(files) == 0 {
			response.BadRequest(ctx, "缺少 file 字段")
			return nil, false
		}
		items := make([]uploadItem, 0, len(files))
		for _, fh := range files {
			if fh.Size == 0 {
				// 禁止空文件
				log.Printf("拒绝空文件 name=%q", fh.Filename)
				continue
			}
			if fh.Size > c.upload.MaxBytes() {
				continue
			}
			data, err := readFileHeader(fh)
			if err != nil {
				continue
			}
			items = append(items, uploadItem{name: fh.Filename, data: data})
		}
		return items, true

	case strings.HasPrefix(ct, "application/json"):
		var req dto.UploadJSONReq
		if err := ctx.ShouldBindJSON(&req); err != nil {
			response.BadRequest(ctx, "参数错误："+err.Error())
			return nil, false
		}
		data, err := base64.StdEncoding.DecodeString(req.Image)
		if err != nil {
			response.BadRequest(ctx, "base64 解码失败")
			return nil, false
		}
		name := req.Filename
		if name == "" {
			name = "upload.png"
		}
		return []uploadItem{{name: name, data: data}}, true

	default:
		data, err := io.ReadAll(io.LimitReader(ctx.Request.Body, c.upload.MaxBytes()+1))
		if err != nil {
			response.BadRequest(ctx, "读取请求体失败")
			return nil, false
		}
		if int64(len(data)) > c.upload.MaxBytes() {
			response.BadRequest(ctx, "文件超过大小限制")
			return nil, false
		}
		return []uploadItem{{name: inferName(ct), data: data}}, true
	}
}

func markdown(url, name string, isImage bool) string {
	if name == "" {
		name = "file"
	}
	if isImage {
		return "![" + escapeMarkdownAlt(name) + "](" + url + ")"
	}
	return "[" + escapeMarkdownAlt(name) + "](" + url + ")"
}

func html(url, name string, isImage bool) string {
	if name == "" {
		name = "file"
	}
	if isImage {
		return `<img src="` + htmlpkg.EscapeString(url) + `" alt="` + htmlpkg.EscapeString(name) + `" />`
	}
	return `<a href="` + htmlpkg.EscapeString(url) + `" target="_blank" rel="noopener">` + htmlpkg.EscapeString(name) + `</a>`
}

func escapeMarkdownAlt(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `[`, `\[`)
	s = strings.ReplaceAll(s, `]`, `\]`)
	s = strings.ReplaceAll(s, `(`, `\(`)
	s = strings.ReplaceAll(s, `)`, `\)`)
	return s
}

func inferName(ct string) string {
	m := ct
	if i := strings.IndexByte(m, ';'); i >= 0 {
		m = strings.TrimSpace(m[:i])
	}
	m = strings.ToLower(m)
	ext := util.MimeToExt(m)
	if ext == "" {
		ext = ".bin"
	}
	return "upload" + ext
}
