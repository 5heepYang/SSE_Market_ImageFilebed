package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Upload    UploadConfig    `mapstructure:"upload"`
	Resumable ResumableConfig `mapstructure:"resumable"`
	Storage   StorageConfig   `mapstructure:"storage"`
	OAuth     OAuthConfig     `mapstructure:"oauth"`
}

type ServerConfig struct {
	Port           string   `mapstructure:"port"`
	Mode           string   `mapstructure:"mode"`
	PublicBaseURL  string   `mapstructure:"public_base_url"`
	FrontendDist   string   `mapstructure:"frontend_dist"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	// PathPrefix 前端挂载前缀，如 /image 或 /sdpy/image（来自 SDPY_PUBLIC_BASE）
	PathPrefix string `mapstructure:"path_prefix"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type UploadConfig struct {
	MaxSizeMB    int `mapstructure:"max_size_mb"`
	ThumbMaxEdge int `mapstructure:"thumb_max_edge"`
}

// ResumableConfig 断点续传/分片上传配置。ChunksDir 用于临时分片落盘，
// MaxSizeMB 限制单文件总大小，MaxChunkMB 限制单个 PATCH 请求体积。
type ResumableConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxChunkMB int    `mapstructure:"max_chunk_mb"`
	ChunksDir  string `mapstructure:"chunks_dir"`
	// MaxConcurrent 单账户同时处于上传中的会话上限（并发总量闸）。
	// 服务端到服务端接入（如集市主站持服务 token）时该账户代表整个模块，可按需调大。
	MaxConcurrent int `mapstructure:"max_concurrent"`
	// AdmissionPerSecond / AdmissionBurst 准入令牌桶：新建上传会话（tus create）
	// 每秒补充的令牌数与桶容量（瞬时突发上限）。
	AdmissionPerSecond int `mapstructure:"admission_per_second"`
	AdmissionBurst     int `mapstructure:"admission_burst"`
	// GlobalMaxConcurrent 全平台（所有账户/模块之和）的在传会话总数上限，
	// 保护共享的物理层（磁盘 / SQLite 写锁 / 缩略图 CPU）。<=0 时视为不限制。
	GlobalMaxConcurrent int `mapstructure:"global_max_concurrent"`
	// CredentialTTLMinutes 短期上传凭证（前端直连 tus 用）的有效期（分钟）。
	CredentialTTLMinutes int `mapstructure:"credential_ttl_minutes"`
	// SessionTTLHours 上传会话/分片在未完成时的 GC 保留时长（小时）。
	SessionTTLHours int `mapstructure:"session_ttl_hours"`
	// ActiveWindowMinutes 并发计数的活跃窗口（分钟）：距上次 PATCH 超过该窗口的会话
	// 不再计入并发额度，避免僵尸会话占满配额。
	ActiveWindowMinutes int `mapstructure:"active_window_minutes"`
}

type StorageConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

// OAuthConfig UniSSO 客户端配置（机器对机器密码校验，无 OAuth 跳转）。
// ClientID / ClientSecret 须在 UniSSO 注册并通过环境变量注入，
// 缺失时视为未配置，登录接口返回 503。
type OAuthConfig struct {
	Issuer       string `mapstructure:"issuer"`        // OIDC issuer
	ClientID     string `mapstructure:"client_id"`     // 应用标识
	ClientSecret string `mapstructure:"client_secret"` // 应用密钥，仅后端持有
}

var Cfg *Config

// Load 读取配置文件并填充默认值；找不到配置文件时使用默认值。
func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	setDefaults()
	// 不要用 viper.AutomaticEnv()：它会把 oauth.app_id 等键自动映射到 OAUTH_APP_ID 这类
	// 裸环境变量，与部署平台自身注入的 OAUTH_*（如 ssemarket-sdpy）冲突。所有环境变量覆盖
	// 统一由 applyEnvOverrides 显式处理（OAuth 变量带 IMAGEBED_ 前缀）。

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		return nil, err
	}
	applyEnvOverrides(&c)
	Cfg = &c
	return &c, nil
}

// applyEnvOverrides 用环境变量覆盖关键配置，便于容器化部署（SSE Deploy 平台约定）。
// OAuth 变量统一加 IMAGEBED_ 前缀，避免与部署平台自身注入的 OAUTH_*（如 ssemarket-sdpy）冲突。
// 优先级：环境变量 > config.yaml > 默认值。
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("PORT"); v != "" {
		c.Server.Port = v
	}
	if v := os.Getenv("SERVER_MODE"); v != "" {
		c.Server.Mode = v
	}
	if v := os.Getenv("PUBLIC_BASE"); v != "" {
		c.Server.PublicBaseURL = v
	} else if v := os.Getenv("SDPY_PUBLIC_URL"); v != "" && isSubdomainPublicURL(v) {
		// 子域名主入口（如 https://image.ssemarket.cn）优先作图片直链根
		c.Server.PublicBaseURL = strings.TrimRight(v, "/")
	} else if v := os.Getenv("SDPY_API_PUBLIC_URL"); v != "" {
		// SSE Deploy 路径模式后端公网地址
		c.Server.PublicBaseURL = v
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		c.Storage.DataDir = v
	} else if v := os.Getenv("SDPY_DATA_DIR"); v != "" {
		c.Storage.DataDir = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.JWT.Secret = v
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_MAX_SIZE_MB"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.MaxSizeMB = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_MAX_CHUNK_MB"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.MaxChunkMB = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_CHUNKS_DIR"); v != "" {
		c.Resumable.ChunksDir = v
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_MAX_CONCURRENT"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.MaxConcurrent = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_ADMISSION_PER_SECOND"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.AdmissionPerSecond = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_ADMISSION_BURST"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.AdmissionBurst = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_GLOBAL_MAX_CONCURRENT"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.GlobalMaxConcurrent = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_CREDENTIAL_TTL_MINUTES"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.CredentialTTLMinutes = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_SESSION_TTL_HOURS"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.SessionTTLHours = n
		}
	}
	if v := os.Getenv("IMAGEBED_RESUMABLE_ACTIVE_WINDOW_MINUTES"); v != "" {
		if n := parseInt(v); n > 0 {
			c.Resumable.ActiveWindowMinutes = n
		}
	}
	if v := os.Getenv("FRONTEND_DIST"); v != "" {
		c.Server.FrontendDist = v
	}
	if v := os.Getenv("IMAGEBED_ALLOWED_ORIGINS"); v != "" {
		c.Server.AllowedOrigins = splitCommaList(v)
	}
	// 子域名模式下不挂 PathPrefix，避免 SPA 仍走 /sdpy/image/
	if isSubdomainPublicURL(c.Server.PublicBaseURL) || isSubdomainPublicURL(os.Getenv("SDPY_PUBLIC_URL")) {
		c.Server.PathPrefix = ""
	} else if v := os.Getenv("SDPY_PUBLIC_BASE"); v != "" {
		c.Server.PathPrefix = v
	} else if v := os.Getenv("PATH_PREFIX"); v != "" {
		c.Server.PathPrefix = v
	}
	if v := os.Getenv("UNISSO_ISSUER"); v != "" {
		c.OAuth.Issuer = v
	}
	if v := os.Getenv("UNISSO_CLIENT_ID"); v != "" {
		c.OAuth.ClientID = v
	}
	if v := os.Getenv("UNISSO_CLIENT_SECRET"); v != "" {
		c.OAuth.ClientSecret = v
	}
}

// isSubdomainPublicURL 判断是否为「主机根」公网 URL（无业务路径），如 https://image.ssemarket.cn
func isSubdomainPublicURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u := strings.TrimRight(raw, "/")
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return false
	}
	rest := u
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	return !strings.Contains(rest, "/")
}

// splitCommaList 解析逗号分隔的环境变量列表。
func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func setDefaults() {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.public_base_url", "")
	viper.SetDefault("server.frontend_dist", "")
	viper.SetDefault("server.allowed_origins", []string{})
	viper.SetDefault("database.path", "data/imagebed.db")
	viper.SetDefault("jwt.secret", "change-me-to-a-long-random-secret-string")
	viper.SetDefault("jwt.expire_hours", 168)
	viper.SetDefault("upload.max_size_mb", 20)
	viper.SetDefault("upload.thumb_max_edge", 400)
	viper.SetDefault("resumable.enabled", true)
	viper.SetDefault("resumable.max_size_mb", 150)
	viper.SetDefault("resumable.max_chunk_mb", 2)
	viper.SetDefault("resumable.chunks_dir", "")
	viper.SetDefault("resumable.max_concurrent", 30)
	viper.SetDefault("resumable.admission_per_second", 5)
	viper.SetDefault("resumable.admission_burst", 20)
	viper.SetDefault("resumable.global_max_concurrent", 100)
	viper.SetDefault("resumable.credential_ttl_minutes", 60)
	viper.SetDefault("resumable.session_ttl_hours", 4)
	viper.SetDefault("resumable.active_window_minutes", 10)
	viper.SetDefault("storage.data_dir", "data")
	viper.SetDefault("oauth.issuer", "https://sso.ssemarket.cn")
	viper.SetDefault("oauth.client_id", "")
	viper.SetDefault("oauth.client_secret", "")
}

func (c *Config) DataDir() string   { return c.Storage.DataDir }
func (c *Config) ImagesDir() string { return filepath.Join(c.Storage.DataDir, "images") }
func (c *Config) ThumbsDir() string { return filepath.Join(c.Storage.DataDir, "thumbs") }

func (c *Config) MaxUploadBytes() int64 {
	return int64(c.Upload.MaxSizeMB) * 1024 * 1024
}

func (c *Config) ResumableMaxBytes() int64 {
	if c.Resumable.MaxSizeMB <= 0 {
		return 150 * 1024 * 1024
	}
	return int64(c.Resumable.MaxSizeMB) * 1024 * 1024
}

func (c *Config) ResumableChunkBytes() int64 {
	if c.Resumable.MaxChunkMB <= 0 {
		return 2 * 1024 * 1024
	}
	return int64(c.Resumable.MaxChunkMB) * 1024 * 1024
}

// ResumableMaxConcurrent 单账户同时上传中的会话上限（未配置时 30）。
func (c *Config) ResumableMaxConcurrent() int {
	if c.Resumable.MaxConcurrent <= 0 {
		return 30
	}
	return c.Resumable.MaxConcurrent
}

// ResumableAdmissionPerSecond 准入令牌桶补充速率（未配置时 5 个/秒）。
func (c *Config) ResumableAdmissionPerSecond() float64 {
	if c.Resumable.AdmissionPerSecond <= 0 {
		return 5
	}
	return float64(c.Resumable.AdmissionPerSecond)
}

// ResumableAdmissionBurst 准入令牌桶容量（未配置时 20）。
func (c *Config) ResumableAdmissionBurst() int {
	if c.Resumable.AdmissionBurst <= 0 {
		return 20
	}
	return c.Resumable.AdmissionBurst
}

// ResumableGlobalMaxConcurrent 全平台在传会话总数上限（未配置时 100；显式希望不限制时设为 -1）。
func (c *Config) ResumableGlobalMaxConcurrent() int {
	if c.Resumable.GlobalMaxConcurrent == 0 {
		return 100
	}
	return c.Resumable.GlobalMaxConcurrent
}

// ResumableCredentialTTL 短期上传凭证有效期（未配置时 60 分钟）。
func (c *Config) ResumableCredentialTTL() time.Duration {
	if c.Resumable.CredentialTTLMinutes <= 0 {
		return 60 * time.Minute
	}
	return time.Duration(c.Resumable.CredentialTTLMinutes) * time.Minute
}

// ResumableSessionTTL 上传会话/分片的 GC 保留时长（未配置时 4 小时）。
func (c *Config) ResumableSessionTTL() time.Duration {
	if c.Resumable.SessionTTLHours <= 0 {
		return 4 * time.Hour
	}
	return time.Duration(c.Resumable.SessionTTLHours) * time.Hour
}

// ResumableActiveWindow 并发计数活跃窗口（未配置时 10 分钟）。
func (c *Config) ResumableActiveWindow() time.Duration {
	if c.Resumable.ActiveWindowMinutes <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.Resumable.ActiveWindowMinutes) * time.Minute
}

func (c *Config) ChunksDir() string {
	if strings.TrimSpace(c.Resumable.ChunksDir) != "" {
		return c.Resumable.ChunksDir
	}
	return filepath.Join(c.Storage.DataDir, "chunks")
}

// PublicURL 由相对存储路径拼出可访问的直链。
// 配置了 public_base_url 时返回绝对地址，否则返回相对地址（/i/...）。
func (c *Config) PublicURL(rel string) string {
	rel = strings.TrimLeft(rel, "/")
	prefix := "/i/"
	if strings.HasPrefix(rel, "objects/") {
		prefix = "/o/"
	}
	base := strings.TrimRight(c.Server.PublicBaseURL, "/")
	if base == "" {
		return prefix + rel
	}
	return base + prefix + rel
}

// OAuthConfigured 是否已配置 UniSSO 客户端凭证（ClientID 与 ClientSecret 同时非空）。
func (c *Config) OAuthConfigured() bool {
	return c.OAuth.ClientID != "" && c.OAuth.ClientSecret != ""
}

// OAuthIssuer 返回去掉末尾斜杠的 issuer。
func (c *Config) OAuthIssuer() string {
	return strings.TrimRight(c.OAuth.Issuer, "/")
}
