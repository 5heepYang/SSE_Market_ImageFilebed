package service

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"time"

	"imagebed/blob"
	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"
	"imagebed/util"

	"github.com/google/uuid"
)

type UploadService struct {
	images *repository.ImageRepo
	cfg    *config.Config
	store  blob.Store
}

func NewUploadService(images *repository.ImageRepo, cfg *config.Config, store blob.Store) *UploadService {
	return &UploadService{images: images, cfg: cfg, store: store}
}

// MaxBytes 单文件上传大小上限，供控制器做早期拦截，避免超大请求读入内存。
func (s *UploadService) MaxBytes() int64 {
	return s.cfg.MaxUploadBytes()
}

// Save 校验并落盘一张图片，写入数据库，返回带直链的记录。
func (s *UploadService) Save(userID uint, originalName string, data []byte) (*model.Image, error) {
	if len(data) == 0 {
		return nil, errors.New("空文件")
	}
	if int64(len(data)) > s.cfg.MaxUploadBytes() {
		return nil, errors.New("文件超过大小限制")
	}

	ext := filepath.Ext(originalName)
	mime, isImage, err := util.ValidateObject(data, ext)
	if err != nil {
		return nil, err
	}
	ext = util.MimeToExt(mime)

	subDir := time.Now().Format("2006/01/02")
	id := uuid.NewString()
	storedName := id + ext

	prefix := "images/"
	if !isImage {
		prefix = "objects/"
	}
	relPath := prefix + subDir + "/" + storedName
	ctx := context.Background()
	if err := s.store.Put(ctx, relPath, bytes.NewReader(data), int64(len(data)), mime); err != nil {
		return nil, err
	}

	width, height := 0, 0
	thumbPath := ""
	if isImage && util.IsRaster(mime) {
		width, height = util.ImageDimensions(data)
		thumbName := id + ".jpg"
		thumbRel := "thumbs/" + subDir + "/" + thumbName
		if s.cfg.Upload.ThumbMaxEdge > 0 {
			if thumbBytes, err := util.GenerateThumbnailBytes(data, s.cfg.Upload.ThumbMaxEdge); err == nil {
				if err := s.store.Put(ctx, thumbRel, bytes.NewReader(thumbBytes), int64(len(thumbBytes)), "image/jpeg"); err == nil {
					thumbPath = thumbRel
				}
			}
		}
	}

	img := &model.Image{
		UserID:       userID,
		OriginalName: originalName,
		Path:         relPath,
		ThumbPath:    thumbPath,
		MimeType:     mime,
		Size:         int64(len(data)),
		Width:        width,
		Height:       height,
		SHA256:       util.SHA256Bytes(data),
		Visibility:   model.VisibilityPublic,
	}
	if err := s.images.Create(img); err != nil {
		_ = s.store.Delete(ctx, relPath)
		if thumbPath != "" {
			_ = s.store.Delete(ctx, thumbPath)
		}
		return nil, err
	}

	img.URL = s.cfg.PublicURL(img.Path)
	if img.ThumbPath != "" {
		img.ThumbURL = s.cfg.PublicURL(img.ThumbPath)
	}
	return img, nil
}
