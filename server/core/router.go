package core

import (
	"imagebed/blob"
	"imagebed/config"
	"imagebed/controller"
	"imagebed/middleware"
	"imagebed/repository"
	"imagebed/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter 组装路由与依赖。
func SetupRouter(cfg *config.Config, db *gorm.DB, store blob.Store) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.MaxBody(cfg.MaxUploadBytes()*5 + (1 << 20)))
	r.MaxMultipartMemory = cfg.MaxUploadBytes()

	userRepo := repository.NewUserRepo(db)
	tokenRepo := repository.NewTokenRepo(db)
	imageRepo := repository.NewImageRepo(db)
	sessionRepo := repository.NewUploadSessionRepo(db)

	authSvc := service.NewAuthService(userRepo, cfg)
	unissoSvc := service.NewUnissoService(userRepo, authSvc, cfg)
	tokenSvc := service.NewTokenService(tokenRepo)
	imageSvc := service.NewImageService(imageRepo, cfg, store)
	uploadSvc := service.NewUploadService(imageRepo, cfg, store)
	resumableSvc := service.NewResumableService(imageRepo, cfg, store)

	authCtrl := controller.NewAuthController(authSvc, unissoSvc)
	tokenCtrl := controller.NewTokenController(tokenSvc)
	imageCtrl := controller.NewImageController(imageSvc, uploadSvc)
	uploadCtrl := controller.NewUploadController(uploadSvc)
	uploadLimiter := newKeyedRateLimiter(5, 10)
	go uploadLimiter.cleanupLoop()
	tokenLimiter := newKeyedRateLimiter(2, 5)
	go tokenLimiter.cleanupLoop()
	loginLimiter := newKeyedRateLimiter(1, 5)
	go loginLimiter.cleanupLoop()

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/login", rateLimitMiddleware(loginLimiter), authCtrl.Login)
			auth.GET("/me", middleware.JWTAuth(authSvc), authCtrl.Me)
		}

		images := api.Group("/images", middleware.JWTAuth(authSvc))
		{
			images.POST("", rateLimitMiddleware(uploadLimiter), imageCtrl.Upload)
			images.GET("", imageCtrl.List)
			images.GET("/:id", imageCtrl.Get)
			images.GET("/:id/file", imageCtrl.File)
			images.PATCH("/:id", imageCtrl.Update)
			images.DELETE("/:id", imageCtrl.Delete)
		}

		tokens := api.Group("/tokens", middleware.JWTAuth(authSvc))
		{
			tokens.POST("", rateLimitMiddleware(tokenLimiter), tokenCtrl.Create)
			tokens.GET("", tokenCtrl.List)
			tokens.DELETE("/:id", rateLimitMiddleware(tokenLimiter), tokenCtrl.Revoke)
		}

		// 短期上传凭证：调用者用自身身份（JWT 或 API Token）换取一张短期 JWT，供前端直连 tus。
		// 注意：不能挂在 /upload/resumable/ 下，gin 的 catch-all /upload/resumable/*path 会与静态段
		// credential 冲突（启动即 panic），故放到 /upload/credential。
		api.POST("/upload/credential", middleware.AnyAuth(authSvc, tokenSvc), authCtrl.IssueCredential)
		api.POST("/upload", middleware.AnyAuth(authSvc, tokenSvc), rateLimitMiddleware(uploadLimiter), uploadCtrl.Upload)
	}

	if err := setupResumable(r, cfg, authSvc, tokenSvc, resumableSvc, sessionRepo); err != nil {
		panic(err)
	}

	r.GET("/i/*filepath", func(c *gin.Context) {
		serveImage(c, store, imageSvc, authSvc, tokenSvc)
	})
	r.GET("/o/*filepath", func(c *gin.Context) {
		serveImage(c, store, imageSvc, authSvc, tokenSvc)
	})

	mountFrontend(r, cfg)
	return r
}
