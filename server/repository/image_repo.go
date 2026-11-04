package repository

import (
	"imagebed/model"

	"gorm.io/gorm"
)

type ImageRepo struct {
	db *gorm.DB
}

func NewImageRepo(db *gorm.DB) *ImageRepo {
	return &ImageRepo{db: db}
}

func (r *ImageRepo) Create(img *model.Image) error {
	return r.db.Create(img).Error
}

// FindByID 只返回属于该用户的图片。
func (r *ImageRepo) FindByID(userID, id uint) (*model.Image, error) {
	var img model.Image
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// FindByPath 按原图相对路径查找（用于直链服务，无用户过滤）。
func (r *ImageRepo) FindByPath(path string) (*model.Image, error) {
	var img model.Image
	if err := r.db.Where("path = ?", path).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// FindByThumbPath 按缩略图相对路径查找。
func (r *ImageRepo) FindByThumbPath(path string) (*model.Image, error) {
	var img model.Image
	if err := r.db.Where("thumb_path = ?", path).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// FindByTusUploadID 用于断点续传完成回调的幂等处理。
func (r *ImageRepo) FindByTusUploadID(tusID string) (*model.Image, error) {
	var img model.Image
	if err := r.db.Where("tus_upload_id = ?", tusID).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *ImageRepo) List(userID uint, keyword, visibility string, page, pageSize int) ([]model.Image, int64, error) {
	q := r.db.Model(&model.Image{}).Where("user_id = ?", userID)
	if keyword != "" {
		q = q.Where("original_name LIKE ?", "%"+keyword+"%")
	}
	if visibility != "" {
		q = q.Where("visibility = ?", visibility)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var imgs []model.Image
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&imgs).Error
	return imgs, total, err
}

func (r *ImageRepo) Update(img *model.Image) error {
	return r.db.Save(img).Error
}

// Delete 仅删除属于该用户的图片记录。
func (r *ImageRepo) Delete(userID, id uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Image{}).Error
}

// CountByUser 统计某用户的图片总数（可用于后续配额）。
func (r *ImageRepo) CountByUser(userID uint) (int64, error) {
	var n int64
	err := r.db.Model(&model.Image{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}
