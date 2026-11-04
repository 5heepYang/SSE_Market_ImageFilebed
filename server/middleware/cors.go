package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
)

var tusExtraHeaders = []string{
    "Tus-Resumable",
    "Upload-Length",
    "Upload-Metadata",
    "Upload-Offset",
    "Upload-Defer-Length",
}

var tusExposeHeaders = []string{
    "Location",
    "Upload-Offset",
    "Tus-Resumable",
    "X-Object-URL",
    "X-Object-Name",
    "X-Object-Mime",
}

var tusAllowedMethods = []string{
    "GET", "POST", "PATCH", "HEAD", "DELETE", "OPTIONS",
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
    allowed := make(map[string]struct{}, len(allowedOrigins))
    for _, origin := range allowedOrigins {
        if origin = strings.TrimSpace(origin); origin != "" {
            allowed[origin] = struct{}{}
        }
    }

    allowHeaders := buildAllowHeaders()
    exposeHeaders := buildExposeHeaders()
    allowMethods := strings.Join(tusAllowedMethods, ", ")

    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        if origin != "" {
            if _, ok := allowed[origin]; ok {
                c.Header("Access-Control-Allow-Origin", origin)
                c.Header("Access-Control-Allow-Credentials", "true")
                c.Header("Vary", "Origin")
            }
            c.Header("Access-Control-Allow-Headers", allowHeaders)
            c.Header("Access-Control-Allow-Methods", allowMethods)
            c.Header("Access-Control-Expose-Headers", exposeHeaders)
        }
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}

func buildAllowHeaders() string {
    base := []string{"Authorization", "Content-Type", "Range"}
    all := append(base, tusExtraHeaders...)
    return strings.Join(all, ", ")
}

func buildExposeHeaders() string {
    base := []string{"Content-Range", "Content-Length", "Content-Type"}
    all := append(base, tusExposeHeaders...)
    return strings.Join(all, ", ")
}
