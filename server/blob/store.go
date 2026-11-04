package blob

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Store 图片/缩略图二进制存储。key 形如 images/2026/08/21/uuid.png（像文件夹路径）。
type Store interface {
	// Backend 返回 local 或 minio，便于日志。
	Backend() string
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// NewFromEnv 优先 MinIO（MINIO_ENDPOINT + AK/SK + BUCKET），否则本地 DATA_DIR。
func NewFromEnv(dataDir string) (Store, error) {
	endpoint := strings.TrimSpace(os.Getenv("MINIO_ENDPOINT"))
	ak := strings.TrimSpace(os.Getenv("MINIO_ACCESS_KEY"))
	sk := strings.TrimSpace(os.Getenv("MINIO_SECRET"))
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if endpoint != "" && ak != "" && sk != "" && bucket != "" {
		return newMinIO(endpoint, ak, sk, bucket)
	}
	return newLocal(dataDir), nil
}

type localStore struct {
	root string
}

func newLocal(root string) *localStore {
	return &localStore{root: root}
}

func (s *localStore) Backend() string { return "local" }

func (s *localStore) abs(key string) (string, error) {
	key = strings.TrimPrefix(filepath.ToSlash(key), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("非法对象键")
	}
	full := filepath.Join(s.root, filepath.FromSlash(key))
	if !strings.HasPrefix(filepath.Clean(full), filepath.Clean(s.root)) {
		return "", fmt.Errorf("路径越界")
	}
	return full, nil
}

func (s *localStore) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	full, err := s.abs(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *localStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	full, err := s.abs(key)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *localStore) Delete(_ context.Context, key string) error {
	full, err := s.abs(key)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

type minioStore struct {
	client *minio.Client
	bucket string
}

func newMinIO(endpoint, ak, sk, bucket string) (*minioStore, error) {
	host, secure, err := parseMinIOEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	cli, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(ak, sk, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	return &minioStore{client: cli, bucket: bucket}, nil
}

func parseMinIOEndpoint(raw string) (host string, secure bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("空 MINIO_ENDPOINT")
	}
	if !strings.Contains(raw, "://") {
		return strings.TrimSuffix(raw, "/"), false, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, err
	}
	host = u.Host
	if host == "" {
		host = u.Path
	}
	return host, u.Scheme == "https", nil
}

func (s *minioStore) Backend() string { return "minio:" + s.bucket }

func (s *minioStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	key = strings.TrimPrefix(key, "/")
	if key == "" || strings.Contains(key, "..") {
		return fmt.Errorf("非法对象键")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *minioStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	key = strings.TrimPrefix(key, "/")
	return s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

func (s *minioStore) Delete(ctx context.Context, key string) error {
	key = strings.TrimPrefix(key, "/")
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
