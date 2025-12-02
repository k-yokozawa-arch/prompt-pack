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
	DeleteObject(ctx context.Context, key string) error
}

type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string]storedObject
}

type storedObject struct {
	body        []byte
	contentType string
	createdAt   time.Time
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{data: map[string]storedObject{}}
}

func (s *InMemoryStorage) PutObject(ctx context.Context, key string, body []byte, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = storedObject{body: body, contentType: contentType, createdAt: time.Now().UTC()}
	return nil
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

func (s *InMemoryStorage) DeleteObject(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	delete(s.data, key)
	return nil
}
