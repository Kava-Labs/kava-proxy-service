package cache

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("value not found in the cache")

type Cache interface {
	// Set sets the value in the cache with specified expiration.
	// Expiration should be either greater than zero or equal to -1, -1 means cache indefinitely.
	Set(ctx context.Context, key string, data []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Healthcheck(ctx context.Context) error
}
