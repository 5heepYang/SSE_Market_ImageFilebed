package service

import (
	"context"
	"errors"
	"io"

	"imagebed/blob"
	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"

	"gorm.io/gorm"
)

var ErrImageNotFound = errors.New("图片不存在")

type ImageService struct {
	images *repository.ImageRepo
	cfg    *config.Config
	store  blob.Store
}

func NewImageService(images *repository.ImageRepo, cfg *config.Config, store blob.Store) *ImageService {
	return &ImageService{images: images, cfg: cfg, store: store}
}

// List 返回当前用户的图片列表（分页 + 搜索 + 可见性过滤）。
func (s *ImageService) List(userID uint, keyword, visibility string, page, pageSize int) ([]model.Image, int64, error) {
	imgs, total, err := s.images.List(userID, keyword, visibility, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	for i := range imgs {
		s.fillURLs(&imgs[i])
	}
	return imgs, total, nil
}

func (s *ImageService) Get(userID, id uint) (*model.Image, error) {
	img, err := s.images.FindByID(userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	s.fillURLs(img)
	return img, nil
}

func (s *ImageService) UpdateVisibility(userID, id uint, visibility string) (*model.Image, error) {
	img, err := s.images.FindByID(userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	if visibility != model.VisibilityPublic && visibility != model.VisibilityPrivate {
		return nil, errors.New("非法的可见性")
	}
	img.Visibility = visibility
	if err := s.images.Update(img); err != nil {
		return nil, err
	}
	s.fillURLs(img)
	return img, nil
}

// Delete 删除图片记录及其原图、缩略图文件。
func (s *ImageService) Delete(userID, id uint) error {
	img, err := s.images.FindByID(userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrImageNotFound
		}
		return err
	}
	if err := s.images.Delete(userID, id); err != nil {
		return err
	}
	ctx := context.Background()
	_ = s.store.Delete(ctx, img.Path)
	if img.ThumbPath != "" {
		_ = s.store.Delete(ctx, img.ThumbPath)
	}
	return nil
}

// FindByPathForServe 供直链服务按相对路径定位图片（无用户过滤，配合可见性控制使用）。
func (s *ImageService) FindByPathForServe(path string) (*model.Image, error) {
	img, err := s.images.FindByPath(path)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	return img, nil
}

func (s *ImageService) FindByThumbPathForServe(path string) (*model.Image, error) {
	img, err := s.images.FindByThumbPath(path)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	return img, nil
}

// Open 打开文件二进制流，供同源文件预览接口使用。
func (s *ImageService) Open(ctx context.Context, img *model.Image) (io.ReadCloser, error) {
	return s.store.Get(ctx, img.Path)
}

func (s *ImageService) fillURLs(img *model.Image) {
	img.URL = s.cfg.PublicURL(img.Path)
	if img.ThumbPath != "" {
		img.ThumbURL = s.cfg.PublicURL(img.ThumbPath)
	}
}
