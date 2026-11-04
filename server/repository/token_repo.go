package repository

import (
	"imagebed/model"
	"time"

	"gorm.io/gorm"
)

type TokenRepo struct {
	db *gorm.DB
}

func NewTokenRepo(db *gorm.DB) *TokenRepo {
	return &TokenRepo{db: db}
}

func (r *TokenRepo) Create(t *model.Token) error {
	return r.db.Create(t).Error
}

func (r *TokenRepo) ListByUser(userID uint) ([]model.Token, error) {
	var tokens []model.Token
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&tokens).Error
	return tokens, err
}

func (r *TokenRepo) FindByHash(hash string) (*model.Token, error) {
	var t model.Token
	if err := r.db.Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// Delete 仅删除属于该用户的 Token，保证不能删别人的。
func (r *TokenRepo) Delete(userID, id uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Token{}).Error
}

func (r *TokenRepo) TouchLastUsed(id uint, at time.Time) error {
	return r.db.Model(&model.Token{}).Where("id = ?", id).Update("last_used_at", at).Error
}
