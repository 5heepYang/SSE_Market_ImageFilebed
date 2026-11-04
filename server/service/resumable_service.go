package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"imagebed/blob"
	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"
	"imagebed/util"

	"gorm.io/gorm"
)

type ResumableService struct {
	images *repository.ImageRepo
	cfg    *config.Config
	store  blob.Store
}

func NewResumableService(images *repository.ImageRepo, cfg *config.Config, store blob.Store) *ResumableService {
	return &ResumableService{images: images, cfg: cfg, store: store}
}

// Finalize 将 tusd 已合并完成的临时文件登记为正式对象。
// 使用 tusID 生成确定性对象路径，重复调用可幂等返回同一条记录。
func (s *ResumableService) Finalize(ctx context.Context, tusID string, userID uint, originalName, chunkPath string) (*model.Image, error) {
	if strings.TrimSpace(tusID) == "" || strings.TrimSpace(chunkPath) == "" {
		return nil, errors.New("非法上传任务")
	}

	if img, err := s.images.FindByTusUploadID(tusID); err == nil {
		s.fillURL(img)
		return img, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	name := sanitizeFilename(originalName)
	ext := strings.ToLower(filepath.Ext(name))
	f, err := os.Open(chunkPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	head := make([]byte, 8)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, errors.New("读取上传文件失败")
	}
	mime, err := util.ValidateDocumentHead(head[:n], ext)
	if err != nil {
		return nil, err
	}

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= 0 {
		return nil, errors.New("空文件")
	}
	if size > s.cfg.ResumableMaxBytes() {
		return nil, errors.New("文件超过大小限制")
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	relPath := fmt.Sprintf("objects/%s/%s%s", time.Now().Format("2006/01/02"), tusID, ext)
	if err := s.store.Put(ctx, relPath, f, size, mime); err != nil {
		return nil, err
	}

	img := &model.Image{
		UserID:       userID,
		OriginalName: name,
		Path:         relPath,
		TusUploadID:  tusID,
		MimeType:     mime,
		Size:         size,
		Visibility:   model.VisibilityPublic,
	}
	if err := s.images.Create(img); err != nil {
		_ = s.store.Delete(ctx, relPath)
		return nil, err
	}

	_ = os.Remove(chunkPath)
	s.fillURL(img)
	return img, nil
}

func (s *ResumableService) fillURL(img *model.Image) {
	img.URL = s.cfg.PublicURL(img.Path)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return '_'
		}
		return r
	}, name)
	if name == "" || name == "." {
		return "file"
	}
	runes := []rune(name)
	if len(runes) > 120 {
		name = string(runes[:120])
	}
	return name
}
