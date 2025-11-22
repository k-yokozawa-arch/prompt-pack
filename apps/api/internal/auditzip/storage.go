package auditzip

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"
)

type Storage interface {
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	GetSignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{data: map[string][]byte{}}
}

func (s *InMemoryStorage) PutObject(ctx context.Context, key string, body []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = body
	return ctx.Err()
}

func (s *InMemoryStorage) GetSignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data[key]; !ok {
		return "", fmt.Errorf("not found")
	}
	exp := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	u := url.URL{Scheme: "https", Host: "storage.local", Path: "/" + key, RawQuery: "exp=" + url.QueryEscape(exp)}
	return u.String(), nil
}
