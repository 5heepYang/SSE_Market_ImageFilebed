package util

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

var allowedImageTypes = map[string]bool{
	"image/jpeg":    true,
	"image/png":     true,
	"image/gif":     true,
	"image/webp":    true,
	"image/bmp":     true,
	"image/svg+xml": true,
}

var extToMime = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
}

var documentExtToMime = map[string]string{
	".pdf":  "application/pdf",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

var mimeToExt = map[string]string{
	"image/jpeg":         ".jpg",
	"image/png":          ".png",
	"image/gif":          ".gif",
	"image/webp":         ".webp",
	"image/bmp":          ".bmp",
	"image/svg+xml":      ".svg",
	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
	"application/vnd.ms-excel": ".xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.ms-powerpoint":                                             ".ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
}

// ValidateImage 根据文件内容（嗅探）与扩展名校验是否为受支持的图片类型，
// 返回规范化后的 MIME。安全要点：内容嗅探为已知但不受支持的类型（如 text/html）时一律拒绝，
// 即使伪装了 .png 扩展名也无法绕过。
func ValidateImage(data []byte, ext string) (string, error) {
	if len(data) == 0 {
		return "", errors.New("空文件")
	}
	ext = strings.ToLower(ext)

	sniffed := http.DetectContentType(data)
	sniffMime := sniffed
	if i := strings.IndexByte(sniffed, ';'); i >= 0 {
		sniffMime = strings.TrimSpace(sniffed[:i])
	}

	// 内容嗅探已命中白名单 → 以嗅探结果为准。
	if allowedImageTypes[sniffMime] {
		return sniffMime, nil
	}

	// SVG：DetectContentType 常返回 text/xml，需结合扩展名放行。
	if ext == ".svg" && (sniffMime == "text/xml" || sniffMime == "image/svg+xml") {
		return "image/svg+xml", nil
	}

	// 内容无法识别（无特征字节）时，退回按扩展名判定。
	if sniffMime == "application/octet-stream" || sniffMime == "text/plain" {
		if m, ok := extToMime[ext]; ok {
			return m, nil
		}
	}

	return "", errors.New("不支持的文件类型")
}

// ValidateObject 校验图片或文档，返回规范 MIME 及是否为图片。
// 文档基于扩展名 + 文件头双重校验，避免把伪装文件当文档存储。
func ValidateObject(data []byte, ext string) (string, bool, error) {
	if len(data) == 0 {
		return "", false, errors.New("空文件")
	}
	if mime, err := ValidateImage(data, ext); err == nil {
		return mime, true, nil
	}
	mime, err := validateDocument(data, ext)
	if err != nil {
		return "", false, errors.New("不支持的文件类型")
	}
	return mime, false, nil
}

func validateDocument(data []byte, ext string) (string, error) {
	ext = strings.ToLower(ext)
	mime, ok := documentExtToMime[ext]
	if !ok {
		return "", errors.New("不支持的文档类型")
	}

	switch mime {
	case "application/pdf":
		if len(data) < 4 || !bytes.Equal(data[:4], []byte("%PDF")) {
			return "", errors.New("PDF 文件头无效")
		}
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		if len(data) < 4 || !bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
			return "", errors.New("Office 文件头无效")
		}
	case "application/msword", "application/vnd.ms-excel", "application/vnd.ms-powerpoint":
		if len(data) < 8 || !bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
			return "", errors.New("Office 文件头无效")
		}
	}
	return mime, nil
}

// ValidateDocumentHead 对文档文件头做快速校验，供流式上传完成后使用。
func ValidateDocumentHead(data []byte, ext string) (string, error) {
	return validateDocument(data, ext)
}

// MimeToExt 返回 MIME 对应的规范扩展名。
func MimeToExt(mime string) string {
	if e, ok := mimeToExt[mime]; ok {
		return e
	}
	return ""
}

// MimeByExt 返回扩展名对应的规范 MIME。
func MimeByExt(ext string) string {
	ext = strings.ToLower(ext)
	if m, ok := extToMime[ext]; ok {
		return m
	}
	if m, ok := documentExtToMime[ext]; ok {
		return m
	}
	return ""
}

// ContentTypeForServe 输出文件响应时使用的 Content-Type。
// 历史数据可能把 MIME 存成空值或 application/octet-stream，此时按扩展名兜底，
// 避免浏览器把 PDF 等文档当成下载文件（另存为）。
func ContentTypeForServe(storedMime, rel string) string {
	mimeType := strings.TrimSpace(storedMime)
	if mimeType != "" && mimeType != "application/octet-stream" {
		return mimeType
	}
	if ext := filepath.Ext(rel); ext != "" {
		if detected := MimeByExt(ext); detected != "" {
			return detected
		}
	}
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

// IsImageMime 判断 MIME 是否为图片，用于文档链接/响应分支。
func IsImageMime(mime string) bool {
	return allowedImageTypes[mime]
}

// IsAllowedDocumentExt 判断扩展名是否为受支持的文档类型。
func IsAllowedDocumentExt(ext string) bool {
	_, ok := documentExtToMime[strings.ToLower(ext)]
	return ok
}

// IsRaster 判断是否为位图（SVG 为矢量图，不参与缩略图/尺寸探测）。
func IsRaster(mime string) bool {
	return mime != "image/svg+xml"
}

// ImageDimensions 从图片头部读取宽高，失败时返回 0,0。
func ImageDimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// GenerateThumbnail 生成缩略图（保持比例，最长边不超过 maxEdge），输出为 JPEG。
func GenerateThumbnail(srcPath, dstPath string, maxEdge int) error {
	src, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	thumb := imaging.Fit(src, maxEdge, maxEdge, imaging.Lanczos)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	return imaging.Save(thumb, dstPath, imaging.JPEGQuality(82))
}

// GenerateThumbnailBytes 从内存图片生成 JPEG 缩略图（供 MinIO 等非本地盘路径）。
func GenerateThumbnailBytes(data []byte, maxEdge int) ([]byte, error) {
	src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	thumb := imaging.Fit(src, maxEdge, maxEdge, imaging.Lanczos)
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumb, imaging.JPEG, imaging.JPEGQuality(82)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ResizeParams 图片按需缩放参数。Width / Height 为 0 时表示该边不限制。
// Format 支持 "jpg"/"jpeg"/"png"，留空时 PNG 保持 PNG，其余输出 JPEG。
type ResizeParams struct {
	Width   int
	Height  int
	Quality int
	Format  string
}

// ResizeImageBytes 从原始图片字节生成指定尺寸的图片。
// 返回处理后的字节与 MIME 类型。
func ResizeImageBytes(data []byte, p ResizeParams) ([]byte, string, error) {
	if p.Width <= 0 && p.Height <= 0 {
		return nil, "", errors.New("请指定 w 或 h 参数")
	}
	if p.Width > 4096 || p.Height > 4096 {
		return nil, "", errors.New("图片尺寸过大")
	}

	src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", err
	}

	var dst image.Image
	if p.Width > 0 && p.Height > 0 {
		dst = imaging.Fit(src, p.Width, p.Height, imaging.Lanczos)
	} else if p.Width > 0 {
		dst = imaging.Resize(src, p.Width, 0, imaging.Lanczos)
	} else {
		dst = imaging.Resize(src, 0, p.Height, imaging.Lanczos)
	}

	format := strings.ToLower(strings.TrimSpace(p.Format))
	if format == "" {
		format = "jpeg"
	}

	var buf bytes.Buffer
	switch format {
	case "png":
		if err := imaging.Encode(&buf, dst, imaging.PNG); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	case "jpg", "jpeg":
		q := p.Quality
		if q <= 0 {
			q = 85
		}
		if q > 100 {
			q = 100
		}
		if err := imaging.Encode(&buf, dst, imaging.JPEG, imaging.JPEGQuality(q)); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	default:
		return nil, "", errors.New("不支持的图片输出格式")
	}
}
