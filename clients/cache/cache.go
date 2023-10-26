package cache

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("value not found in the cache")

type Cache interface {
	Set(ctx context.Context, key string, data []byte, expiration time.Duration, cacheIndefinitely bool) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Healthcheck(ctx context.Context) error
}
