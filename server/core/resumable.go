package core

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imagebed/config"
	"imagebed/middleware"
	"imagebed/repository"
	"imagebed/service"
	"imagebed/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tus/tusd/v2/pkg/filestore"
	"github.com/tus/tusd/v2/pkg/handler"
	"github.com/tus/tusd/v2/pkg/memorylocker"
)

func setupResumable(r *gin.Engine, cfg *config.Config, authSvc *service.AuthService, tokenSvc *service.TokenService, resumableSvc *service.ResumableService, sessions *repository.UploadSessionRepo) error {
	if !cfg.Resumable.Enabled {
		return nil
	}

	chunksDir := cfg.ChunksDir()
	if err := os.MkdirAll(chunksDir, 0o755); err != nil {
		return err
	}

	composer := handler.NewStoreComposer()
	store := filestore.New(chunksDir)
	store.UseIn(composer)
	locker := memorylocker.New()
	locker.UseIn(composer)

	pathPrefix := strings.TrimSuffix(strings.TrimSpace(cfg.Server.PathPrefix), "/")
	routePrefix := "/api/upload/resumable"
	basePath := pathPrefix + routePrefix + "/"

	tusConfig := handler.Config{
		StoreComposer:           composer,
		MaxSize:                 cfg.ResumableMaxBytes(),
		BasePath:                basePath,
		RespectForwardedHeaders: true,
		DisableDownload:         true,
		PreUploadCreateCallback: func(hook handler.HookEvent) (handler.HTTPResponse, handler.FileInfoChanges, error) {
			return tusPreCreate(hook, sessions, cfg.ResumableMaxConcurrent(), cfg.ResumableGlobalMaxConcurrent(), time.Now().Add(-cfg.ResumableActiveWindow()))
		},
		PreFinishResponseCallback: func(hook handler.HookEvent) (handler.HTTPResponse, error) {
			return tusPreFinish(hook, resumableSvc, sessions)
		},
		PreUploadTerminateCallback: func(hook handler.HookEvent) (handler.HTTPResponse, error) {
			return tusPreTerminate(hook, sessions)
		},
	}
	tusHandler, err := handler.NewHandler(tusConfig)
	if err != nil {
		return err
	}

	stripHandler := http.StripPrefix(routePrefix, tusHandler)
	// 准入令牌桶：只作用于新建上传会话（POST），容量/速率来自配置（默认 20 个、5 个/秒）
	admission := newKeyedRateLimiter(cfg.ResumableAdmissionPerSecond(), cfg.ResumableAdmissionBurst())
	// 保护性限流：作用于分片 PATCH / HEAD / DELETE 等请求，取宽松值避免卡分片吞吐
	general := newKeyedRateLimiter(100, 200)
	go admission.cleanupLoop()
	go general.cleanupLoop()
	go startResumableCleanup(chunksDir, sessions, cfg.ResumableSessionTTL())

	r.Any("/api/upload/resumable/*path", middleware.AnyAuth(authSvc, tokenSvc), tusRateLimitMiddleware(admission, general), func(c *gin.Context) {
		uid := middleware.UserID(c)
		c.Request.Header.Set("X-Authenticated-User-ID", strconv.FormatUint(uint64(uid), 10))
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.ResumableChunkBytes())
		// PATCH 分片视为会话活跃：刷新 updated_at，使活跃窗口并发计数生效（僵尸会话回收）。
		if c.Request.Method == http.MethodPatch {
			if tusID := tusIDFromPath(c.Param("path")); tusID != "" {
				_ = sessions.Touch(tusID) // 失败不阻塞上传，GC 兜底
			}
		}
		stripHandler.ServeHTTP(c.Writer, c.Request)
	})
	return nil
}

func tusPreCreate(hook handler.HookEvent, sessions *repository.UploadSessionRepo, maxConcurrent, globalMax int, activeSince time.Time) (handler.HTTPResponse, handler.FileInfoChanges, error) {
	uidStr := hook.HTTPRequest.Header.Get("X-Authenticated-User-ID")
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil || uid == 0 {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_UNAUTHORIZED", "未登录或 Token 无效", http.StatusUnauthorized)
	}

	if maxConcurrent <= 0 {
		maxConcurrent = 30
	}
	active, err := sessions.CountActive(uint(uid), activeSince)
	if err != nil {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INTERNAL", "检查上传任务失败", http.StatusInternalServerError)
	}
	if int(active) >= maxConcurrent {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_TOO_MANY_UPLOADS", fmt.Sprintf("同时上传的文件不能超过 %d 个", maxConcurrent), http.StatusTooManyRequests)
	}

	// 全局兜底：所有账户（模块）的在传总数封顶，保护共享的物理层（磁盘 / SQLite 写锁 / 缩略图 CPU）。
	// 各模块的令牌池互相独立，只有这道闸是全平台共享的；显式设为负值可关闭。
	if globalMax > 0 {
		total, err := sessions.CountAll(activeSince)
		if err != nil {
			return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INTERNAL", "检查上传任务失败", http.StatusInternalServerError)
		}
		if int(total) >= globalMax {
			return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_PLATFORM_BUSY", "当前平台上传任务较多，请稍后再试", http.StatusTooManyRequests)
		}
	}

	// 禁止空文件（Upload-Length 为 0 或缺失/延迟声明）
	if hook.Upload.Size <= 0 {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_EMPTY_FILE", "不能上传空文件", http.StatusBadRequest)
	}

	filename := strings.TrimSpace(hook.Upload.MetaData["filename"])
	const maxFilenameLen = 200
	if len([]rune(filename)) > maxFilenameLen {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_FILENAME_TOO_LONG", "文件名过长", http.StatusBadRequest)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if filename == "" || !util.IsAllowedDocumentExt(ext) {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_UNSUPPORTED_TYPE", "仅支持 pdf/doc/docx/xls/xlsx/ppt/pptx", http.StatusBadRequest)
	}

	tusID := uuid.NewString()
	if err := sessions.Create(tusID, uint(uid)); err != nil {
		return handler.HTTPResponse{}, handler.FileInfoChanges{}, handler.NewError("ERR_INTERNAL", "创建上传任务失败", http.StatusInternalServerError)
	}

	changes := handler.FileInfoChanges{
		ID: tusID,
		MetaData: handler.MetaData{
			"user_id":  uidStr,
			"filename": filename,
		},
	}
	return handler.HTTPResponse{}, changes, nil
}

func tusPreFinish(hook handler.HookEvent, resumableSvc *service.ResumableService, sessions *repository.UploadSessionRepo) (handler.HTTPResponse, error) {
	uid, err := strconv.ParseUint(hook.Upload.MetaData["user_id"], 10, 64)
	if err != nil || uid == 0 {
		return handler.HTTPResponse{}, handler.NewError("ERR_UNAUTHORIZED", "上传任务缺少用户信息", http.StatusUnauthorized)
	}

	filename := hook.Upload.MetaData["filename"]
	chunkPath := hook.Upload.Storage[filestore.StorageKeyPath]
	img, err := resumableSvc.Finalize(hook.Context, hook.Upload.ID, uint(uid), filename, chunkPath)
	if err != nil {
		return handler.HTTPResponse{}, handler.NewError("ERR_OBJECT_STORE", err.Error(), http.StatusInternalServerError)
	}
	_ = sessions.Delete(hook.Upload.ID)

	return handler.HTTPResponse{
		Header: handler.HTTPHeader{
			"X-Object-URL":  img.URL,
			"X-Object-Name": img.OriginalName,
			"X-Object-Mime": img.MimeType,
		},
	}, nil
}

func tusPreTerminate(hook handler.HookEvent, sessions *repository.UploadSessionRepo) (handler.HTTPResponse, error) {
	_ = sessions.Delete(hook.Upload.ID)
	return handler.HTTPResponse{}, nil
}

func startResumableCleanup(dir string, sessions *repository.UploadSessionRepo, ttl time.Duration) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	// cleanOnce 清理残留的分片与垃圾数据：
	//  1. 分片目录中超过 TTL 未完成的数据/信息文件 → 删除文件并注销对应会话；
	//  2. 数据库中已过期但仍残留的会话记录（分片文件已丢失或从未落盘）→ 直接删除，
	//     避免其永久占用用户的并发上传额度。
	cleanOnce := func() {
		cutoff := time.Now().Add(-ttl)
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
				_ = os.Remove(path)
				id := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
				_ = sessions.Delete(id)
			}
			return nil
		})
		_ = sessions.DeleteExpired(cutoff)
		// 观测：清理后输出各用户活跃会话数，为并发参数调优提供真实分布（无活跃会话时不输出）
		if byUser, err := sessions.ActiveByUser(); err == nil {
			var total int64
			parts := make([]string, 0, len(byUser))
			for uid, n := range byUser {
				total += n
				parts = append(parts, fmt.Sprintf("user=%d:%d", uid, n))
			}
			if total > 0 {
				log.Printf("活跃上传会话 total=%d %s", total, strings.Join(parts, " "))
			}
		}
	}
	// 启动时先清理一次（处理上次运行遗留的半成品分片），之后每小时例行执行
	cleanOnce()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		cleanOnce()
	}
}

// tusIDFromPath 从 gin 通配参数（形如 /<tusID>）提取上传会话 ID。
func tusIDFromPath(p string) string {
	return strings.Trim(strings.TrimSpace(p), "/")
}
