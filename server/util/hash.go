package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Bytes 计算字节内容的小写十六进制 SHA-256。
func SHA256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// RandomHex 生成 n 字节加密安全随机数的小写十六进制字符串（长度 2n）。
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
