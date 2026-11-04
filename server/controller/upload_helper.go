package controller

import (
	"io"
	"mime/multipart"
)

// readFileHeader 读取 multipart 文件内容到内存。
func readFileHeader(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
