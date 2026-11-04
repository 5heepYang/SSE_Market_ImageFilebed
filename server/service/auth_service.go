package service

import (
	"errors"
	"time"

	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidOrExpire = errors.New("登录态无效或已过期")

type Claims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

type AuthService struct {
	users *repository.UserRepo
	cfg   *config.Config
}

func NewAuthService(users *repository.UserRepo, cfg *config.Config) *AuthService {
	return &AuthService{users: users, cfg: cfg}
}

// IssueToken 为本地用户签发 Web 会话 JWT（有效期由 jwt.expire_hours 决定）。
func (s *AuthService) IssueToken(u *model.User) (string, error) {
	return s.IssueTokenWithTTL(u, time.Duration(s.cfg.JWT.ExpireHours)*time.Hour)
}

// IssueTokenWithTTL 为本地用户签发指定有效期的 JWT。
// 短期上传凭证（前端直连 tus）即走此方法，有效期远短于 Web 会话态。
func (s *AuthService) IssueTokenWithTTL(u *model.User, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: u.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.OAuthID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWT.Secret))
}

// IssueUploadCredential 为上传场景签发短期凭证，返回 token 与有效期（秒）。
// 有效期取自 resumable.credential_ttl_minutes，远短于 Web 会话态。
func (s *AuthService) IssueUploadCredential(u *model.User) (string, int, error) {
	ttl := s.cfg.ResumableCredentialTTL()
	token, err := s.IssueTokenWithTTL(u, ttl)
	if err != nil {
		return "", 0, err
	}
	return token, int(ttl.Seconds()), nil
}

// ParseToken 校验 JWT 并返回用户 ID。
func (s *AuthService) ParseToken(tokenStr string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("非预期的签名方法")
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if err != nil {
		return 0, ErrInvalidOrExpire
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, ErrInvalidOrExpire
	}
	return claims.UserID, nil
}

func (s *AuthService) GetUser(id uint) (*model.User, error) {
	return s.users.FindByID(id)
}
