package storage

import "context"

type Storage interface {
	Load(ctx context.Context, key string) (string, error)
	Store(ctx context.Context, key string, value string) error
	StoreWithConversation(ctx context.Context, key string, value string, conversationID string) error
	DeleteByConversation(ctx context.Context, conversationID string) error
}

type MemoryStorage struct {
	data map[string]string
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string]string),
	}
}

func (s *MemoryStorage) Load(ctx context.Context, key string) (string, error) {
	return s.data[key], nil
}

func (s *MemoryStorage) Store(ctx context.Context, key string, value string) error {
	s.data[key] = value
	return nil
}

func (s *MemoryStorage) StoreWithConversation(ctx context.Context, key string, value string, conversationID string) error {
	s.data[key] = value
	return nil
}

func (s *MemoryStorage) DeleteByConversation(ctx context.Context, conversationID string) error {
	return nil
}
