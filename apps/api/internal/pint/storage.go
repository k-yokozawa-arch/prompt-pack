package pint

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"
)

type ObjectMeta struct {
	Key       string
	Size      int
	UpdatedAt time.Time
}

type Storage interface {
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	GetSignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	Head(ctx context.Context, key string) (ObjectMeta, error)
}

// InMemoryStorage is a lightweight stub to unblock local testing without S3.
type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string][]byte
	meta map[string]ObjectMeta
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: map[string][]byte{},
		meta: map[string]ObjectMeta{},
	}
}

func (s *InMemoryStorage) PutObject(ctx context.Context, key string, body []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = body
	s.meta[key] = ObjectMeta{
		Key:       key,
		Size:      len(body),
		UpdatedAt: time.Now().UTC(),
	}
	return ctx.Err()
}

func (s *InMemoryStorage) GetSignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.data[key]; !ok {
		return "", fmt.Errorf("not found")
	}
	exp := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	u := url.URL{
		Scheme:   "https",
		Host:     "storage.local",
		Path:     "/" + key,
		RawQuery: "exp=" + url.QueryEscape(exp),
	}
	return u.String(), nil
}

func (s *InMemoryStorage) Head(_ context.Context, key string) (ObjectMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.meta[key]
	if !ok {
		return ObjectMeta{}, fmt.Errorf("not found")
	}
	return meta, nil
}
