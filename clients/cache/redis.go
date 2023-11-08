package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

// RedisCache is an implementation of Cache that uses Redis as the caching backend.
type RedisCache struct {
	client *redis.Client
	*logging.ServiceLogger
}

var _ Cache = (*RedisCache)(nil)

func NewRedisCache(
	cfg *RedisConfig,
	logger *logging.ServiceLogger,
) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &RedisCache{
		client:        client,
		ServiceLogger: logger,
	}, nil
}

// Set sets the value for the given key in the cache with the given expiration.
func (rc *RedisCache) Set(
	ctx context.Context,
	key string,
	value []byte,
	expiration time.Duration,
) error {
	rc.Logger.Trace().
		Str("key", key).
		Str("value", string(value)).
		Dur("expiration", expiration).
		Msg("setting value in redis")

	// -1 means cache indefinitely.
	if expiration == -1 {
		// In redis zero expiration means the key has no expiration time.
		expiration = 0
	}

	return rc.client.Set(ctx, key, value, expiration).Err()
}

// Get gets the value for the given key in the cache.
func (rc *RedisCache) Get(
	ctx context.Context,
	key string,
) ([]byte, error) {
	rc.Logger.Trace().
		Str("key", key).
		Msg("getting value from redis")

	val, err := rc.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		rc.Logger.Trace().
			Str("key", key).
			Msgf("value not found in redis")
		return nil, ErrNotFound
	}
	if err != nil {
		rc.Logger.Error().
			Str("key", key).
			Err(err).
			Msg("error during getting value from redis")
		return nil, err
	}

	rc.Logger.Trace().
		Str("key", key).
		Str("value", string(val)).
		Msg("successfully got value from redis")

	return val, nil
}

// Delete deletes the value for the given key in the cache.
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	rc.Logger.Trace().
		Str("key", key).
		Msg("deleting value from redis")

	return rc.client.Del(ctx, key).Err()
}

func (rc *RedisCache) Healthcheck(ctx context.Context) error {
	rc.Logger.Trace().Msg("redis healthcheck was called")

	// Check if we can connect to Redis
	_, err := rc.client.Ping(ctx).Result()
	if err != nil {
		rc.Logger.Error().
			Err(err).
			Msg("can't ping redis")
		return fmt.Errorf("error connecting to Redis: %v", err)
	}

	rc.Logger.Trace().Msg("redis healthcheck was successful")

	return nil
}
