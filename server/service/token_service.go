package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"imagebed/model"
	"imagebed/repository"
	"imagebed/util"
)

var (
	ErrTokenNotFound = errors.New("Token 不存在")
	ErrTokenExpired  = errors.New("Token 已过期")
)

type TokenService struct {
	tokens *repository.TokenRepo
}

func NewTokenService(tokens *repository.TokenRepo) *TokenService {
	return &TokenService{tokens: tokens}
}

// Create 生成一个新的 API Token，返回明文（仅此一次）与记录。
func (s *TokenService) Create(userID uint, name string, expiresAt *time.Time) (string, *model.Token, error) {
	raw, err := randomToken()
	if err != nil {
		return "", nil, err
	}
	t := &model.Token{
		UserID:    userID,
		Name:      name,
		TokenHash: util.SHA256Bytes([]byte(raw)),
		ExpiresAt: expiresAt,
	}
	if err := s.tokens.Create(t); err != nil {
		return "", nil, err
	}
	return raw, t, nil
}

func (s *TokenService) List(userID uint) ([]model.Token, error) {
	return s.tokens.ListByUser(userID)
}

func (s *TokenService) Revoke(userID, id uint) error {
	return s.tokens.Delete(userID, id)
}

// Authenticate 校验 Token 明文，成功则返回其记录并刷新最近使用时间。
func (s *TokenService) Authenticate(raw string) (*model.Token, error) {
	if raw == "" {
		return nil, ErrTokenNotFound
	}
	hash := util.SHA256Bytes([]byte(raw))
	t, err := s.tokens.FindByHash(hash)
	if err != nil {
		return nil, ErrTokenNotFound
	}
	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}
	_ = s.tokens.TouchLastUsed(t.ID, time.Now())
	return t, nil
}

// randomToken 生成 "imgb_" 前缀的随机 Token。
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "imgb_" + hex.EncodeToString(b), nil
}
