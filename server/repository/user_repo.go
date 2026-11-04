package repository

import (
	"imagebed/model"

	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(u *model.User) error {
	return r.db.Create(u).Error
}

func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var u model.User
	if err := r.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) FindByOAuthID(oauthID string) (*model.User, error) {
	var u model.User
	if err := r.db.Where("oauth_id = ?", oauthID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByEmail 按 email 精确匹配查找用户（存量账号合并判定用）。
func (r *UserRepo) FindByEmail(email string) (*model.User, error) {
	var u model.User
	if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateOAuthID 把旧集市用户 ID 迁移为 UniSSO sub。
func (r *UserRepo) UpdateOAuthID(id uint, oauthID string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).
		Update("oauth_id", oauthID).Error
}

// UpdateProfile 同步 OAuth 拉取到的用户资料。
func (r *UserRepo) UpdateProfile(id uint, name, email, avatar string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "email": email, "avatar_url": avatar}).Error
}
