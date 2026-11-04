package service

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"imagebed/config"
	"imagebed/model"
	"imagebed/repository"

	"gorm.io/gorm"
)

var (
	// ErrNotInternal 账号未通过院内身份校验（UniSSO internal 非 true）。
	ErrNotInternal = errors.New("该账号没有图床访问权限，仅限软件工程学院内部账号使用")
	// ErrInvalidSSO 共享会话不存在 / 已撤销 / 已过期。
	ErrInvalidSSO = errors.New("invalid_sso")
	// ErrSSOUnavailable UniSSO 限流、故障或网络异常（静默回落表单，不踢本地会话）。
	ErrSSOUnavailable = errors.New("sso_unavailable")
)

const (
	passwordVerifyPath = "/api/oauth/password/verify"
	sessionVerifyPath  = "/api/oauth/session/verify"
	sessionRevokePath  = "/api/oauth/session/revoke"

	// SSOCookieName 共享 SSO Cookie 的新名称；unisso 以 password/verify 响应里的
	// sso_cookie_name 为真源，这里只是响应缺省时的兜底。
	SSOCookieName = "unisso_sid"
	// SSOCookieLegacyName 旧版共享 Cookie 名（unisso 切换默认名前已签发的存量），
	// 兼容期双读双清。
	SSOCookieLegacyName = "sse_sso"

	fallbackSSOCookieDomain = ".ssemarket.cn"
	fallbackSSOMaxAge       = 604800
)

// SSOCookie UniSSO 签发的共享会话 Cookie 元数据（由登录平台种到父域供全平台静默兑换）。
type SSOCookie struct {
	Session string
	Name    string
	Domain  string
	MaxAge  int
}

// UnissoService 通过 UniSSO 机器对机器密码校验端点完成邮箱+密码登录（无 OAuth 跳转）：
// POST {issuer}/api/oauth/password/verify（form：email/password/client_id/client_secret），
// 成功 200 {"ok":true,"user":{sub,sso_sub,sse_sub,internal,...},sso_session,sso_cookie_name,...}；
// 按 sub upsert 本地用户。响应里的共享会话元数据用于在同一响应中种父域 Cookie。
type UnissoService struct {
	cfg   *config.Config
	users *repository.UserRepo
	auth  *AuthService
	http  *http.Client
}

// UnissoUser UniSSO 返回的用户资料。
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
// 成功时同时返回共享会话 Cookie 元数据（可能为 nil，视 unisso 响应而定）。
func (s *UnissoService) Verify(email, password string) (*UnissoUser, *SSOCookie, error) {
	form := url.Values{}
	form.Set("email", strings.ToLower(strings.TrimSpace(email)))
	form.Set("password", password)
	form.Set("client_id", s.cfg.OAuth.ClientID)
	form.Set("client_secret", s.cfg.OAuth.ClientSecret)

	resp, body, err := s.doRequest(http.MethodPost, s.cfg.OAuthIssuer()+passwordVerifyPath,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, nil, errors.New("无法连接登录服务，请稍后再试")
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// unisso 侧按 IP+email 锁 15 分钟，自动重试只会更糟
		return nil, nil, errors.New("尝试次数过多，请 15 分钟后再试")
	case resp.StatusCode == http.StatusBadRequest:
		return nil, nil, errors.New("请填写邮箱和密码")
	case resp.StatusCode == http.StatusForbidden:
		return nil, nil, errors.New("登录服务拒绝本应用，请检查客户端配置")
	case resp.StatusCode == http.StatusUnauthorized:
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &e) == nil {
			switch e.Error {
			case "invalid_credentials":
				return nil, nil, errors.New("邮箱或密码错误")
			case "invalid_client":
				return nil, nil, errors.New("登录服务配置错误，请联系管理员")
			}
		}
		return nil, nil, errors.New("登录失败，请稍后再试")
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, nil, errors.New("登录失败，请稍后再试")
	}

	var out struct {
		OK              bool       `json:"ok"`
		User            UnissoUser `json:"user"`
		SSOSession      string     `json:"sso_session"`
		SSOCookieName   string     `json:"sso_cookie_name"`
		SSOCookieDomain string     `json:"sso_cookie_domain"`
		SSOMaxAge       int        `json:"sso_max_age"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, errors.New("登录服务返回异常")
	}
	if !out.OK || out.User.Sub == "" {
		return nil, nil, errors.New("登录服务返回异常")
	}
	if !bool(out.User.Internal) {
		return nil, nil, ErrNotInternal
	}
	var sso *SSOCookie
	if out.SSOSession != "" {
		sso = &SSOCookie{
			Session: out.SSOSession,
			Name:    out.SSOCookieName,
			Domain:  out.SSOCookieDomain,
			MaxAge:  out.SSOMaxAge,
		}
		if sso.Name == "" {
			sso.Name = SSOCookieName
		}
		if sso.Domain == "" {
			sso.Domain = fallbackSSOCookieDomain
		}
		if sso.MaxAge <= 0 {
			sso.MaxAge = fallbackSSOMaxAge
		}
	}
	return &out.User, sso, nil
}

// VerifySession 用共享 SSO Cookie 的值向 UniSSO 校验会话并取用户（静默 SSO）。
// 400/401 → ErrInvalidSSO；其余非 2xx、结构异常、网络故障 → ErrSSOUnavailable；
// 非 internal → ErrNotInternal。
func (s *UnissoService) VerifySession(session string) (*UnissoUser, error) {
	form := url.Values{}
	form.Set("session", session)
	form.Set("client_id", s.cfg.OAuth.ClientID)
	form.Set("client_secret", s.cfg.OAuth.ClientSecret)

	resp, body, err := s.doRequest(http.MethodPost, s.cfg.OAuthIssuer()+sessionVerifyPath,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, ErrSSOUnavailable
	}
	switch {
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrInvalidSSO
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, ErrSSOUnavailable
	}

	var out struct {
		OK   bool       `json:"ok"`
		User UnissoUser `json:"user"`
	}
	if err := json.Unmarshal(body, &out); err != nil || !out.OK || out.User.Sub == "" {
		return nil, ErrSSOUnavailable
	}
	if !bool(out.User.Internal) {
		return nil, ErrNotInternal
	}
	return &out.User, nil
}

// RevokeSession 撤销共享会话（登出用，尽力而为：失败只记日志，不阻塞本地登出）。
func (s *UnissoService) RevokeSession(session string) {
	form := url.Values{}
	form.Set("session", session)
	form.Set("client_id", s.cfg.OAuth.ClientID)
	form.Set("client_secret", s.cfg.OAuth.ClientSecret)
	if _, _, err := s.doRequest(http.MethodPost, s.cfg.OAuthIssuer()+sessionRevokePath,
		strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"); err != nil {
		log.Printf("unisso session/revoke 失败（忽略）：%v", err)
	}
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
