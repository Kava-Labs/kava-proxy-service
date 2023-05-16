package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/redis/go-redis/v9"
)

// RedisCache is an implementation of Cache that uses Redis as the caching backend.
type RedisCache struct {
	client *redis.Client
	logger *logging.ServiceLogger
}

var _ Cache = (*RedisCache)(nil)

func NewRedisCache(
	ctx context.Context,
	logger *logging.ServiceLogger,
	address string,
	password string,
	db int,
) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})

	// Check if we can connect to Redis
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("error connecting to Redis: %v", err)
	}

	logger.Logger.Debug().Msg("connected to Redis")

	return &RedisCache{
		client: client,
		logger: logger,
	}, nil
}

// Set sets the value for the given key in the cache with the given expiration.
func (rc *RedisCache) Set(
	ctx context.Context,
	key string,
	value []byte,
	expiration time.Duration,
) error {
	return rc.client.Set(ctx, key, value, expiration).Err()
}

// Get gets the value for the given key in the cache.
func (rc *RedisCache) Get(
	ctx context.Context,
	key string,
) ([]byte, bool) {
	val, err := rc.client.Get(ctx, key).Bytes()

	// Mo error == found. This throws away any other errors that aren't "key not
	// found" errors.
	return val, err == nil
}

// Delete deletes the value for the given key in the cache.
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}
