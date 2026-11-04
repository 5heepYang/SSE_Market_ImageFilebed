package core

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"imagebed/blob"
	"imagebed/config"
	"imagebed/middleware"
	"imagebed/model"
	"imagebed/service"
	"imagebed/util"

	"github.com/gin-gonic/gin"
)

// serveImage 图片直链服务：按相对存储路径输出文件，private 图需要鉴权且为本人。
// 支持按需缩放：GET /i/...?w=800&h=0&q=85&fmt=jpg
func serveImage(c *gin.Context, store blob.Store, imageSvc *service.ImageService, authSvc *service.AuthService, tokenSvc *service.TokenService) {
	rel := strings.TrimPrefix(c.Param("filepath"), "/")
	if rel == "" || strings.Contains(rel, "..") {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	img, err := imageSvc.FindByPathForServe(rel)
	if err != nil {
		img, err = imageSvc.FindByThumbPathForServe(rel)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	if img.Visibility == model.VisibilityPrivate {
		uid, ok := resolveUser(c, authSvc, tokenSvc)
		if !ok || uid != img.UserID {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	contentType := img.MimeType
	var rc io.ReadCloser
	servedRel := rel

	if params, ok := parseResizeParams(c, img.MimeType); ok {
		rc, contentType, servedRel = openResized(c, store, rel, img.MimeType, params)
	}
	if rc == nil {
		rc, err = store.Get(c.Request.Context(), rel)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	defer rc.Close()

	writeImageResponse(c, img, contentType, rc, servedRel)
}

type resizeParams struct {
	width   int
	height  int
	quality int
	format  string
}

func parseResizeParams(c *gin.Context, mimeType string) (resizeParams, bool) {
	w, _ := strconv.Atoi(c.Query("w"))
	h, _ := strconv.Atoi(c.Query("h"))
	if w <= 0 && h <= 0 {
		return resizeParams{}, false
	}
	q, _ := strconv.Atoi(c.Query("q"))
	format := strings.ToLower(strings.TrimSpace(c.Query("fmt")))
	if format == "" {
		if mimeType == "image/png" {
			format = "png"
		} else {
			format = "jpg"
		}
	}
	if format != "jpg" && format != "jpeg" && format != "png" {
		format = "jpg"
	}
	return resizeParams{width: w, height: h, quality: q, format: format}, true
}

func openResized(c *gin.Context, store blob.Store, originalRel, mimeType string, params resizeParams) (io.ReadCloser, string, string) {
	if !strings.HasPrefix(mimeType, "image/") || !util.IsRaster(mimeType) {
		return nil, "", originalRel
	}

	variantRel := resizeCacheKey(originalRel, params)
	lock := resizeLockFor(variantRel)
	lock.Lock()
	defer lock.Unlock()

	if rc, err := store.Get(c.Request.Context(), variantRel); err == nil {
		return rc, resizeContentType(params.format), variantRel
	}

	rc, err := store.Get(c.Request.Context(), originalRel)
	if err != nil {
		return nil, "", originalRel
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", originalRel
	}

	out, contentType, err := util.ResizeImageBytes(data, util.ResizeParams{
		Width:   params.width,
		Height:  params.height,
		Quality: params.quality,
		Format:  params.format,
	})
	if err != nil {
		return nil, "", originalRel
	}

	if putErr := store.Put(c.Request.Context(), variantRel, bytes.NewReader(out), int64(len(out)), contentType); putErr != nil {
		// 缓存写入失败时仍可返回本次生成结果。
		return io.NopCloser(bytes.NewReader(out)), contentType, originalRel
	}
	return io.NopCloser(bytes.NewReader(out)), contentType, variantRel
}

var resizeLocks sync.Map

func resizeLockFor(key string) *sync.Mutex {
	mu := &sync.Mutex{}
	actual, _ := resizeLocks.LoadOrStore(key, mu)
	return actual.(*sync.Mutex)
}

func resizeCacheKey(rel string, params resizeParams) string {
	base := strings.TrimSuffix(rel, filepath.Ext(rel))
	ext := ".jpg"
	if params.format == "png" {
		ext = ".png"
	}
	return fmt.Sprintf("cache/%s_w%d_h%d_q%d%s", base, params.width, params.height, params.quality, ext)
}

func resizeContentType(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}

func writeImageResponse(c *gin.Context, img *model.Image, contentType string, rc io.Reader, servedRel string) {
	if strings.HasPrefix(img.MimeType, "image/svg+xml") {
		c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	}

	if !strings.HasPrefix(img.MimeType, "image/") {
		disposition := "inline"
		if c.Query("download") == "1" {
			disposition = "attachment"
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		cacheControl := "public, max-age=86400"
		if img.Visibility == model.VisibilityPrivate {
			cacheControl = "private, no-store"
		}
		util.ServeFileBytes(c.Writer, c.Request, img.OriginalName, data,
			util.ContentTypeForServe(contentType, servedRel),
			contentDisposition(img.OriginalName, disposition),
			cacheControl)
		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	if img.Visibility == model.VisibilityPrivate {
		c.Header("Cache-Control", "private, no-store")
	} else {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
	c.Header("Content-Type", util.ContentTypeForServe(contentType, servedRel))
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

func contentDisposition(filename, disposition string) string {
	if strings.TrimSpace(filename) == "" {
		filename = "file"
	}
	filename = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return '_'
		}
		return r
	}, filename)
	return mime.FormatMediaType(disposition, map[string]string{"filename": filename})
}

// resolveUser 依次尝试 JWT、API Token（头或 ?token= 查询参数）解析出用户 ID。
func resolveUser(c *gin.Context, authSvc *service.AuthService, tokenSvc *service.TokenService) (uint, bool) {
	raw := middleware.BearerToken(c)
	if raw == "" {
		raw = c.Query("token")
	}
	if raw == "" {
		return 0, false
	}
	if uid, err := authSvc.ParseToken(raw); err == nil {
		return uid, true
	}
	if t, err := tokenSvc.Authenticate(raw); err == nil {
		return t.UserID, true
	}
	return 0, false
}

// mountFrontend 若前端构建产物存在，则托管静态资源并提供 SPA 回退。
// 同时挂 /assets 与 {PathPrefix}/assets：网关 strip 前缀走前者，直连 :23xxx 走后者。
func mountFrontend(r *gin.Engine, cfg *config.Config) {
	dist := cfg.Server.FrontendDist
	if dist == "" {
		return
	}
	abs, err := filepath.Abs(dist)
	if err != nil {
		return
	}
	index := filepath.Join(abs, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}

	assetsDir := filepath.Join(abs, "assets")
	r.Static("/assets", assetsDir)
	// 网关 strip /image 后浏览器仍请求 BASE_URL+logo.png → 上游实际路径为 /logo.png
	logoPath := filepath.Join(abs, "logo.png")
	if _, err := os.Stat(logoPath); err == nil {
		r.StaticFile("/logo.png", logoPath)
	}

	prefix := strings.TrimSuffix(strings.TrimSpace(cfg.Server.PathPrefix), "/")
	if prefix != "" && prefix != "/" {
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		r.Static(prefix+"/assets", assetsDir)
		// 直连 :23xxx/image/… 时仍带前缀
		if _, err := os.Stat(logoPath); err == nil {
			r.StaticFile(prefix+"/logo.png", logoPath)
		}
		r.GET(prefix, func(c *gin.Context) { c.Redirect(http.StatusFound, prefix+"/") })
		r.GET(prefix+"/", func(c *gin.Context) { c.File(index) })
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/i/") || strings.HasPrefix(p, "/o/") {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		rel := p
		if prefix != "" && prefix != "/" && strings.HasPrefix(p, prefix+"/") {
			rel = strings.TrimPrefix(p, prefix)
		}
		full := filepath.Join(abs, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
		// 静态资源：有文件就吐出；没有再 404（切勿 SPA 回退成 HTML）
		if looksLikeStaticAsset(p) {
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				c.File(full)
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if p != "/" && p != prefix && p != prefix+"/" {
			if info, err := os.Stat(full); err == nil && !info.IsDir() {
				c.File(full)
				return
			}
		}
		c.File(index)
	})
}

func looksLikeStaticAsset(p string) bool {
	lower := strings.ToLower(p)
	for _, ext := range []string{".js", ".css", ".mjs", ".map", ".woff", ".woff2", ".ttf", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".ico"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
