package storage

import (
	"context"
	"os"
	"path/filepath"
)

type FileSystemStorage struct {
	baseDir string
}

func NewFileSystemStorage(baseDir string) *FileSystemStorage {
	return &FileSystemStorage{
		baseDir: baseDir,
	}
}

func (fs *FileSystemStorage) Store(ctx context.Context, key string, value string) error {
	filePath := filepath.Clean(filepath.Join(fs.baseDir, key))
	// 如果目录不存在，创建目录
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	err := os.WriteFile(filePath, []byte(value), 0644)
	return err
}

func (fs *FileSystemStorage) Load(ctx context.Context, key string) (string, error) {
	filePath := filepath.Clean(filepath.Join(fs.baseDir, key))
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (fs *FileSystemStorage) StoreWithConversation(ctx context.Context, key string, value string, conversationID string) error {
	return fs.Store(ctx, key, value)
}

func (fs *FileSystemStorage) DeleteByConversation(ctx context.Context, conversationID string) error {
	return nil
}
