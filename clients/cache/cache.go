package cache

import (
	"context"
	"time"
)

// Cache is an interface for caching key-value pairs.
type Cache interface {
	Set(ctx context.Context, key string, data []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
