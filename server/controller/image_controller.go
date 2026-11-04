package controller

import (
	"errors"
	"io"
	"log"
	"mime"
	"strconv"
	"strings"

	"imagebed/dto"
	"imagebed/middleware"
	"imagebed/model"
	"imagebed/response"
	"imagebed/service"
	"imagebed/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ImageController struct {
	svc    *service.ImageService
	upload *service.UploadService
}

func NewImageController(svc *service.ImageService, upload *service.UploadService) *ImageController {
	return &ImageController{svc: svc, upload: upload}
}

// Upload 前端 multipart 多文件上传。
func (c *ImageController) Upload(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	form, err := ctx.MultipartForm()
	if err != nil {
		response.BadRequest(ctx, "解析上传表单失败")
		return
	}
	files := form.File["file"]
	if len(files) == 0 {
		response.BadRequest(ctx, "未选择文件")
		return
	}

	var uploaded []*model.Image
	var failed int
	for _, fh := range files {
		if fh.Size == 0 {
			// 禁止空文件
			failed++
			log.Printf("拒绝空文件 user=%d name=%q", uid, fh.Filename)
			continue
		}
		if fh.Size > c.upload.MaxBytes() {
			failed++
			continue
		}
		data, err := readFileHeader(fh)
		if err != nil {
			failed++
			log.Printf("读取上传文件失败 user=%d name=%q: %v", uid, fh.Filename, err)
			continue
		}
		img, err := c.upload.Save(uid, fh.Filename, data)
		if err != nil {
			failed++
			log.Printf("保存上传文件失败 user=%d name=%q: %v", uid, fh.Filename, err)
			continue
		}
		uploaded = append(uploaded, img)
	}

	if len(uploaded) == 0 {
		response.BadRequest(ctx, "全部文件上传失败")
		return
	}
	if failed > 0 {
		response.OKWithMessage(ctx, uploaded, "部分文件上传失败")
		return
	}
	response.OK(ctx, uploaded)
}

func (c *ImageController) List(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "24"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 24
	}
	keyword := strings.TrimSpace(ctx.Query("keyword"))
	visibility := strings.TrimSpace(ctx.Query("visibility"))

	imgs, total, err := c.svc.List(uid, keyword, visibility, page, pageSize)
	if err != nil {
		log.Printf("图片列表查询失败 user=%d: %v", uid, err)
		response.ServerError(ctx, "服务器内部错误")
		return
	}
	response.OK(ctx, dto.PageData{List: imgs, Total: total, Page: page, PageSize: pageSize})
}

func (c *ImageController) Get(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ctx, "非法 ID")
		return
	}
	img, err := c.svc.Get(uid, uint(id))
	if err != nil {
		handleImageErr(ctx, err)
		return
	}
	response.OK(ctx, img)
}

// File 输出文件二进制内容，供前端同源预览（PDF / DOCX / XLSX / PPTX）。
func (c *ImageController) File(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ctx, "非法 ID")
		return
	}
	img, err := c.svc.Get(uid, uint(id))
	if err != nil {
		handleImageErr(ctx, err)
		return
	}

	rc, err := c.svc.Open(ctx.Request.Context(), img)
	if err != nil {
		response.NotFound(ctx, "文件不存在")
		return
	}
	defer rc.Close()

	disposition := "inline"
	if ctx.Query("download") == "1" {
		disposition = "attachment"
	}
	data, err := io.ReadAll(rc)
	if err != nil {
		response.NotFound(ctx, "文件读取失败")
		return
	}
	util.ServeFileBytes(ctx.Writer, ctx.Request, img.OriginalName, data,
		util.ContentTypeForServe(img.MimeType, img.Path),
		contentDispositionHeader(img.OriginalName, disposition),
		"private, no-store")
}

func contentDispositionHeader(filename, disposition string) string {
	if strings.TrimSpace(filename) == "" {
		filename = "file"
	}
	filename = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return '_'
		}
		return r
	}, filename)
	return mime.FormatMediaType(disposition, map[string]string{"filename": filename})
}

func (c *ImageController) Update(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ctx, "非法 ID")
		return
	}
	var req dto.UpdateImageReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Printf("图片更新参数错误 user=%d: %v", uid, err)
		response.BadRequest(ctx, "参数错误")
		return
	}
	img, err := c.svc.UpdateVisibility(uid, uint(id), req.Visibility)
	if err != nil {
		handleImageErr(ctx, err)
		return
	}
	response.OK(ctx, img)
}

func (c *ImageController) Delete(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ctx, "非法 ID")
		return
	}
	if err := c.svc.Delete(uid, uint(id)); err != nil {
		handleImageErr(ctx, err)
		return
	}
	response.OK(ctx, nil)
}

func handleImageErr(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrImageNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		response.NotFound(ctx, "图片不存在")
	default:
		log.Printf("图片操作失败: %v", err)
		response.ServerError(ctx, "服务器内部错误")
	}
}
