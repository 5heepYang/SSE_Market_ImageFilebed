package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"

	"gorm.io/gorm"
)

// ErrNotInternal 账号未通过院内身份校验（UniSSO internal 非 true）。
var ErrNotInternal = errors.New("该账号没有图床访问权限，仅限软件工程学院内部账号使用")

const passwordVerifyPath = "/api/oauth/password/verify"

// UnissoService 通过 UniSSO 机器对机器密码校验端点完成邮箱+密码登录（无 OAuth 跳转）：
// POST {issuer}/api/oauth/password/verify（form：email/password/client_id/client_secret），
// 成功 200 {"ok":true,"user":{sub,sso_sub,sse_sub,internal,...}}；按 sub upsert 本地用户。
type UnissoService struct {
	cfg   *config.Config
	users *repository.UserRepo
	auth  *AuthService
	http  *http.Client
}

// UnissoUser UniSSO password/verify 返回的用户资料。
type UnissoUser struct {
	Sub               string   `json:"sub"`
	SsoSub            string   `json:"sso_sub"`
	SseSub            string   `json:"sse_sub"`
	Internal          flexBool `json:"internal"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Email             string   `json:"email"`
	Picture           string   `json:"picture"`
	StudentID         string   `json:"student_id"`
}

// flexBool 兼容 UniSSO 把布尔值序列化成 true / 1 / "true" 的情况。
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.ToLower(strings.Trim(string(data), `"`))
	switch s {
	case "true", "1":
		*b = true
	case "false", "0", "", "null":
		*b = false
	default:
		return errors.New("无法解析布尔值: " + string(data))
	}
	return nil
}

// DisplayName 展示名兜底：name → preferred_username。
func (u *UnissoUser) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.PreferredUsername
}

func NewUnissoService(users *repository.UserRepo, auth *AuthService, cfg *config.Config) *UnissoService {
	return &UnissoService{
		cfg:   cfg,
		users: users,
		auth:  auth,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *UnissoService) IsConfigured() bool {
	return s.cfg.OAuthConfigured()
}

// Verify 调 UniSSO 密码校验端点；只做一次调用，429 不自动重试。
func (s *UnissoService) Verify(email, password string) (*UnissoUser, error) {
	form := url.Values{}
	form.Set("email", strings.ToLower(strings.TrimSpace(email)))
	form.Set("password", password)
	form.Set("client_id", s.cfg.OAuth.ClientID)
	form.Set("client_secret", s.cfg.OAuth.ClientSecret)

	resp, body, err := s.doRequest(http.MethodPost, s.cfg.OAuthIssuer()+passwordVerifyPath,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, errors.New("无法连接登录服务，请稍后再试")
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// unisso 侧按 IP+email 锁 15 分钟，自动重试只会更糟
		return nil, errors.New("尝试次数过多，请 15 分钟后再试")
	case resp.StatusCode == http.StatusBadRequest:
		return nil, errors.New("请填写邮箱和密码")
	case resp.StatusCode == http.StatusForbidden:
		return nil, errors.New("登录服务拒绝本应用，请检查客户端配置")
	case resp.StatusCode == http.StatusUnauthorized:
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil {
			switch e.Error {
			case "invalid_credentials":
				return nil, errors.New("邮箱或密码错误")
			case "invalid_client":
				return nil, errors.New("登录服务配置错误，请联系管理员")
			}
		}
		return nil, errors.New("登录失败，请稍后再试")
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, errors.New("登录失败，请稍后再试")
	}

	var out struct {
		OK   bool       `json:"ok"`
		User UnissoUser `json:"user"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, errors.New("登录服务返回异常")
	}
	if !out.OK || out.User.Sub == "" {
		return nil, errors.New("登录服务返回异常")
	}
	if !bool(out.User.Internal) {
		return nil, ErrNotInternal
	}
	return &out.User, nil
}

// UpsertUser 按 UniSSO sub 创建或更新本地用户并返回。
// 首次登录时若存量用户的 email 与本次登录 email 精确相等，复用该行并把
// oauth_id 改写为 sub（图片/Token 归属保留）；否则新建。
func (s *UnissoService) UpsertUser(u *UnissoUser) (*model.User, error) {
	local, err := s.users.FindByOAuthID(u.Sub)
	if err == nil {
		if e := s.users.UpdateProfile(local.ID, u.DisplayName(), u.Email, u.Picture); e != nil {
			return nil, e
		}
		local.Name, local.Email, local.AvatarURL = u.DisplayName(), u.Email, u.Picture
		return local, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if u.Email != "" {
		legacy, err := s.users.FindByEmail(u.Email)
		if err == nil {
			if e := s.users.UpdateOAuthID(legacy.ID, u.Sub); e != nil {
				return nil, e
			}
			if e := s.users.UpdateProfile(legacy.ID, u.DisplayName(), u.Email, u.Picture); e != nil {
				return nil, e
			}
			legacy.OAuthID = u.Sub
			legacy.Name, legacy.Email, legacy.AvatarURL = u.DisplayName(), u.Email, u.Picture
			return legacy, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	nu := &model.User{OAuthID: u.Sub, Name: u.DisplayName(), Email: u.Email, AvatarURL: u.Picture}
	if err := s.users.Create(nu); err != nil {
		return nil, err
	}
	return nu, nil
}

// FinishLogin 为本地用户签发会话 JWT。
func (s *UnissoService) FinishLogin(u *model.User) (string, error) {
	return s.auth.IssueToken(u)
}

func (s *UnissoService) doRequest(method, rawURL string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, b, nil
}
