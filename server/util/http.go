package util

import (
	"bytes"
	"net/http"
	"time"
)

// ServeFileBytes 输出文件响应，支持 Range 分段请求（浏览器内置 PDF 阅读器依赖）。
func ServeFileBytes(w http.ResponseWriter, r *http.Request, name string, data []byte, contentType, disposition, cacheControl string) {
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
